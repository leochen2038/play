package packers

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
	"github.com/leochen2038/play/codec/protos/golang/json"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	ModeKeyValue = "kv"
	ModeJSON     = "json"
)

type TelnetPacker struct {
	//cmdPrefix    string
	paramPrefix  string
	lineDelim    []byte
	maxLineSize  int
	enableTelnet bool
	requestPool  *sync.Pool
}

func NewTelnetPacker() play.IPacker {
	return &TelnetPacker{
		//cmdPrefix:    "-action",
		paramPrefix:  "-",
		lineDelim:    []byte("\r\n"),
		maxLineSize:  2048,
		enableTelnet: true,
		requestPool: &sync.Pool{
			New: func() interface{} {
				return &TelnetRequest{
					Params: make(map[string]interface{}, 8),
				}
			},
		},
	}
}

type TelnetRequest struct {
	CmdURL     string                 `json:"cmd_url"`
	Params     map[string]interface{} `json:"params"`
	RawCommand string                 `json:"raw_command"`
	ParamMode  string                 `json:"param_mode"` // 新增模式标识
	ConnVer    byte                   `json:"version"`    // 原始输入
}

func (r *TelnetRequest) GetURL() string                    { return r.CmdURL }
func (r *TelnetRequest) GetParams() map[string]interface{} { return r.Params }
func (r *TelnetRequest) GetRaw() string                    { return r.RawCommand }
func (r *TelnetRequest) Version() byte                     { return r.ConnVer }

func (p *TelnetPacker) acquireRequest() *TelnetRequest {
	req := p.requestPool.Get().(*TelnetRequest)
	// 重置参数
	req.CmdURL = ""
	req.RawCommand = ""
	for k := range req.Params {
		delete(req.Params, k)
	}
	return req
}

func (p *TelnetPacker) releaseRequest(req *TelnetRequest) {
	p.requestPool.Put(req)
}

func (p *TelnetPacker) Unpack(c *play.Conn) (*play.Request, error) {
	const maxCommandLength = 2048

	// 协议过滤
	data := p.filterTelnetData(c.Tcp.Surplus)
	if len(data) == 0 {
		return nil, nil
	}

	// 安全性检查
	if len(data) > maxCommandLength {
		c.Tcp.Surplus = nil
		return nil, errors.New("command too long")
	}

	// 查找命令结束符
	idx := bytes.Index(data, p.lineDelim)
	if idx == -1 {
		return nil, nil // 等待更多数据
	}

	// 提取命令内容
	command := bytes.TrimSpace(data[:idx])
	c.Tcp.Surplus = data[idx+len(p.lineDelim):]

	// 解析命令行参数
	req := p.parseCommandLine(string(command), c.Tcp.Version)
	if req == nil {
		msg := "please input \r\n 类型一 : 'url路由 json字符串' \r\n 类型二 : 'url路由  -参数 参数值' \r\n"
		return nil, errors.New(msg)
	}

	paramsBytes, _ := json.Marshal(req.GetParams())

	return &play.Request{
		Version:     req.Version(),
		ActionName:  strings.ReplaceAll(strings.Trim(req.GetURL(), "/"), "/", "."),
		Deadline:    time.Now().Add(10 * time.Second),
		InputBinder: binders.GetBinderOfJson(paramsBytes),
	}, nil

}

func (p *TelnetPacker) parseCommandLine(input string, connVer byte) *TelnetRequest {
	req := p.acquireRequest()
	req.RawCommand = input
	req.ConnVer = connVer
	req.ParamMode = ModeKeyValue // 默认键值对模式

	// 提取命令和URL
	url, argsPart := p.splitCommandAndArgs(input)
	if url == "" {
		return nil
	}
	req.CmdURL = url

	// 模式识别与解析
	if p.isJSONPayload(argsPart) {
		req.ParamMode = ModeJSON
		if err := p.parseJSONParams(argsPart, req); err != nil {
			log.Printf("JSON解析失败: %v | 原始输入: %s", err, argsPart)
			return nil
		}
	} else {
		p.parseKeyValueParams(argsPart, req)
	}

	return req
}

func (p *TelnetPacker) splitCommandAndArgs(input string) (url, argsPart string) {
	input = strings.Trim(input, " ")
	indx := strings.Index(input, " ")
	if indx == -1 {
		url = input
		return
	}
	url = input[:indx]
	argsPart = input[indx+1:]
	return
}

// 检测是否为JSON负载
func (p *TelnetPacker) isJSONPayload(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "{") ||
		strings.HasPrefix(trimmed, "[") ||
		strings.Contains(trimmed, `":`)
}

// 解析JSON参数
func (p *TelnetPacker) parseJSONParams(input string, req *TelnetRequest) error {
	// 尝试直接解析
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(input), &jsonData); err == nil {
		for k, v := range jsonData {
			req.Params[k] = v
		}
		return nil
	}

	// 尝试在引号内解析
	if quoted, ok := p.extractQuotedJSON(input); ok {
		if err := json.Unmarshal([]byte(quoted), &jsonData); err == nil {
			for k, v := range jsonData {
				req.Params[k] = v
			}
			return nil
		}
	}

	return fmt.Errorf("invalid JSON format")
}

// 提取引号内的JSON
func (p *TelnetPacker) extractQuotedJSON(input string) (string, bool) {
	if len(input) < 2 {
		return "", false
	}

	firstChar := input[0]
	lastChar := input[len(input)-1]

	if (firstChar == '"' && lastChar == '"') ||
		(firstChar == '\'' && lastChar == '\'') {
		return input[1 : len(input)-1], true
	}

	return "", false
}

// 键值对模式解析（支持混合模式）
func (p *TelnetPacker) parseKeyValueParams(input string, req *TelnetRequest) {
	tokens, _ := p.tokenize(input)
	currentParam := ""

	for _, token := range tokens {
		switch {
		case strings.HasPrefix(token, p.paramPrefix):
			// 新参数开始
			paramName := strings.TrimPrefix(token, p.paramPrefix)
			currentParam = paramName
			req.Params[currentParam] = ""

		case currentParam != "":
			// 参数值赋值
			req.Params[currentParam] = token
			currentParam = ""

		default:
			// 可能是JSON片段或混合值
			if p.isJSONPayload(token) {
				p.parseJSONPayloadFragment(token, req)
			} else {
				// 无名参数
				req.Params[fmt.Sprintf("arg_%d", len(req.Params))] = token
			}
		}
	}
}

// 解析JSON片段（混合模式）
func (p *TelnetPacker) parseJSONPayloadFragment(fragment string, req *TelnetRequest) {
	// 尝试解析为JSON对象
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(fragment), &jsonData); err == nil {
		for k, v := range jsonData {
			req.Params[k] = fmt.Sprint(v)
		}
		return
	}

	// 尝试解析为JSON数组
	var jsonArray []interface{}
	if err := json.Unmarshal([]byte(fragment), &jsonArray); err == nil {
		for i, v := range jsonArray {
			req.Params[fmt.Sprintf("item_%d", i)] = fmt.Sprint(v)
		}
		return
	}

	// 无法解析则作为普通值
	req.Params[fmt.Sprintf("raw_%d", len(req.Params))] = fragment
}

func (p *TelnetPacker) tokenize(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	escape := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		switch {
		case escape:
			current.WriteByte(c)
			escape = false

		case c == '\\':
			escape = true

		case inQuote:
			if c == quoteChar {
				inQuote = false
				tokens = append(tokens, current.String())
				current.Reset()
			} else {
				current.WriteByte(c)
			}

		case c == '"' || c == '\'':
			inQuote = true
			quoteChar = c

		case unicode.IsSpace(rune(c)):
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}

		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

func (p *TelnetPacker) filterTelnetData(input []byte) []byte {
	if !p.enableTelnet || len(input) == 0 {
		return input
	}

	const (
		IAC = 0xFF
		SB  = 0xFA
		SE  = 0xF0
	)

	output := make([]byte, 0, len(input))
	state := 0 // 0=正常, 1=IAC, 2=子协商

	for i := 0; i < len(input); i++ {
		b := input[i]

		switch state {
		case 1: // IAC 状态
			if b == SB {
				state = 2
			} else {
				state = 0
			}

		case 2: // 子协商状态
			if b == IAC {
				state = 1
			} else if b == SE {
				state = 0
			}

		default:
			if b == IAC {
				state = 1
			} else {
				output = append(output, b)
			}
		}
	}

	// 安全性截断
	if len(output) > p.maxLineSize {
		output = output[:p.maxLineSize]
	}
	return output
}

func (p *TelnetPacker) Pack(c *play.Conn, res *play.Response) ([]byte, error) {
	// 1. 创建响应缓冲区
	buf := bytes.NewBuffer(nil)

	buf.Write([]byte{'\r', '\n'})
	// 2. 格式化数据
	if res.Output.All() == nil && res.Error != nil {
		buf.Write([]byte(res.Error.Error()))
	} else {
		buf.Write(p.formatJSONResponse(res.Output.All(), int(c.Tcp.Version)))
	}

	// 3. 写入响应数据
	buf.Write([]byte{'\r', '\n'}) // Telnet 响应结束符

	return buf.Bytes(), nil
}

func (p *TelnetPacker) formatJSONResponse(data interface{}, version int) []byte {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	// 版本3+使用更美观的格式化
	if version >= 2 {
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)

		// 尝试美化输出
		if b, err := json.MarshalIndent(data, "", "  "); err == nil {
			return b
		}
	}

	// 基础格式化
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
	return buf.Bytes()
}
