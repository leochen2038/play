package packers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/leochen2038/play/codec/protos/golang/json"
	"github.com/leochen2038/play/codec/renders"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
)

const (
	defaultAction   = "/index"
	defaultRender   = "json"
	defaultFormSize = 512
)

var (
	ErrUndefinedFilePath = errors.New("undefined file path")
	ErrUndefinedPath     = errors.New("undefined path")
	ErrUndefinedRender   = errors.New("undefined http response render")
)

// MIME 类型映射
var contentTypeMap = map[string]string{
	"html": "text/html; charset=utf-8",
	"js":   "text/javascript; charset=utf-8",
	"css":  "text/css; charset=utf-8",
	"json": "application/json; charset=utf-8",
}

type HttpPacker struct {
}

func NewHttpPacker() play.IPacker {
	return &HttpPacker{}
}

func (p *HttpPacker) Unpack(c *play.Conn) (*play.Request, error) {
	var request = new(play.Request)
	request.ActionName, request.RenderName = p.ParseHttpPath(c.Http.Request.URL.Path)
	request.InputBinder = p.ParseHttpInput(c.Http.Request)
	return request, nil
}

func (p *HttpPacker) Pack(c *play.Conn, res *play.Response) ([]byte, error) {
	// 设置通用 header
	header := c.Http.ResponseWriter.Header()

	// JSON 处理
	if res.RenderName == "json" {
		header.Set("Content-Type", contentTypeMap["json"])
		header.Set("Cache-Control", "no-cache, must-revalidate, max-age=0")
		return renders.GetRenderOfJson().Render(res.Output.All())
	}

	// 静态文件处理
	if contentType, ok := contentTypeMap[res.RenderName]; ok {
		header.Set("Content-Type", contentType)
		return p.handleStaticFile(c, res)
	}

	return nil, fmt.Errorf("%w: %s", ErrUndefinedRender, res.RenderName)
}

func (p *HttpPacker) handleStaticFile(c *play.Conn, res *play.Response) ([]byte, error) {
	if res.TemplateRoot == "" {
		c.Http.ResponseWriter.WriteHeader(http.StatusNotFound)
		return nil, ErrUndefinedFilePath
	}

	// 使用 strings.Builder 优化字符串拼接
	var sb strings.Builder
	sb.Grow(len(res.TemplateRoot) + len(res.Template) + len(res.RenderName) + 2)
	sb.WriteString(strings.TrimRight(res.TemplateRoot, "/"))
	sb.WriteByte('/')
	sb.WriteString(res.Template)
	sb.WriteByte('.')
	sb.WriteString(res.RenderName)

	content, err := os.ReadFile(sb.String())
	if err != nil {
		c.Http.ResponseWriter.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("%w: %s", ErrUndefinedPath, c.Http.Request.URL.Path)
	}

	if res.RenderName == "html" && len(res.Output.All()) > 0 {
		return p.injectHtmlData(content, res)
	}

	return content, nil
}

func (p *HttpPacker) injectHtmlData(content []byte, res *play.Response) ([]byte, error) {
	outputData, err := json.MarshalEscape(res.Output.All(), false, false)
	if err != nil {
		return nil, fmt.Errorf("marshal output data: %w", err)
	}

	// 使用 buffer pool
	buf := bytes.NewBuffer(make([]byte, 0, len(content)+len(outputData)+50))

	// 查找最后的 </html> 标签
	index := bytes.LastIndex(content, []byte("</html>"))
	if index == -1 {
		return content, nil
	}

	// 写入内容
	buf.Write(content[:index])
	buf.WriteString("<script>var initData = ")
	buf.Write(outputData)
	buf.WriteString("</script>\n</html>")

	return buf.Bytes(), nil
}

func (p *HttpPacker) ParseHttpPath(path string) (action, render string) {
	if path == "/" || path == "" {
		return "index", defaultRender
	}

	if indexDot := strings.IndexByte(path, '.'); indexDot > 0 {
		render = path[indexDot+1:]
		path = path[:indexDot]
	} else {
		render = defaultRender
	}

	// 预分配 builder
	var sb strings.Builder
	sb.Grow(len(path))

	// 手动替换 '/' 为 '.'，避免使用 strings.ReplaceAll
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			sb.WriteByte('.')
		} else {
			sb.WriteByte(path[i])
		}
	}

	return sb.String(), render
}

func (p *HttpPacker) ParseHttpInput(request *http.Request) binders.Binder {
	contentType := request.Header.Get("Content-Type")

	switch {
	case strings.Contains(contentType, "/json"),
		strings.Contains(contentType, "/bytes"):
		return p.handleBodyContent(request, contentType)

	case strings.Contains(contentType, "/x-www-form-urlencoded"):
		_ = request.ParseForm()
		return binders.GetBinderOfUrlValue(request.Form, nil)

	case strings.Contains(contentType, "/form-data"):
		_ = request.ParseMultipartForm(defaultFormSize)
		return binders.GetBinderOfUrlValue(request.Form, request.MultipartForm.File)
	}

	return binders.GetBinderOfUrlValue(request.URL.Query(), nil)
}

func (p *HttpPacker) handleBodyContent(request *http.Request, contentType string) binders.Binder {
	// 使用 buffer pool 读取请求体
	buf := bytes.NewBuffer(make([]byte, 0, 512))

	// 直接复制到 buffer，避免中间分配
	if _, err := io.Copy(buf, request.Body); err != nil {
		return binders.GetBinderOfBytes(nil)
	}
	request.Body.Close()

	// 重新设置请求体
	request.Body = io.NopCloser(buf)

	if strings.Contains(contentType, "/json") {
		return binders.GetBinderOfJson(buf.Bytes())
	}
	return binders.GetBinderOfBytes(buf.Bytes())
}
