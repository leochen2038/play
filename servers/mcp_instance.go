package servers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
)

type mcpInstance struct {
	info        play.IInstanceInfo
	hook        play.IServerHook
	ctrl        *play.InstanceCtrl
	packer      play.IPacker
	mcpServer   *mcp.Server
	actions     map[string]*play.ActionUnit
	sortedNames []string
	transport   int
	httpHandler http.Handler
}

func NewMCPInstance(name string, addr string, hook play.IServerHook, transport int, defaultActionTimeout time.Duration) *mcpInstance {
	if hook == nil {
		hook = defaultHook{}
	}
	if defaultActionTimeout == 0 {
		defaultActionTimeout = defaultTimeout
	}
	mcpSrv := mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: "1.0.0",
	}, nil)
	return &mcpInstance{
		info:      play.NewInstanceInfo(name, addr, play.SERVER_TYPE_MCP, defaultActionTimeout),
		hook:      hook,
		packer:    &mcpPacker{},
		ctrl:      new(play.InstanceCtrl),
		mcpServer: mcpSrv,
		actions:   make(map[string]*play.ActionUnit),
		transport: transport,
	}
}

func (i *mcpInstance) MCPServer() *mcp.Server {
	return i.mcpServer
}

func (i *mcpInstance) Info() play.IInstanceInfo  { return i.info }
func (i *mcpInstance) Hook() play.IServerHook    { return i.hook }
func (i *mcpInstance) Ctrl() *play.InstanceCtrl  { return i.ctrl }
func (i *mcpInstance) Packer() play.IPacker      { return i.packer }
func (i *mcpInstance) ActionUnitNames() []string { return append([]string(nil), i.sortedNames...) }
func (i *mcpInstance) Close()                    { i.ctrl.WaitTask() }

func (i *mcpInstance) LookupActionUnit(name string) *play.ActionUnit {
	return i.actions[name]
}

func (i *mcpInstance) Network() string {
	if i.transport == play.MCP_TRANSPORT_STDIO {
		return "stdio"
	}
	return "tcp"
}

func (i *mcpInstance) Transport(conn *play.Conn, data []byte) error {
	conn.Mcp.Data = data
	return nil
}

func (i *mcpInstance) BindActionSpace(spaceName string, actionPackages ...string) error {
	if err := bindActionSpace(i, spaceName, actionPackages); err != nil {
		return err
	}
	for _, unit := range i.actions {
		tool := &mcp.Tool{
			Name:        unit.RequestName,
			Description: unit.Action.MetaData()["desc"],
			InputSchema: actionFieldsToSchema(unit.Action.Input()),
		}
		i.mcpServer.AddTool(tool, i.makeToolHandler(unit))
	}
	return nil
}

func (i *mcpInstance) makeToolHandler(unit *play.ActionUnit) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sess := play.NewSession(ctx, i)
		playReq := &play.Request{
			ActionName: unit.RequestName,
		}
		if req.Params.Arguments != nil {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err == nil && args != nil {
				playReq.InputBinder = binders.GetBinderOfMap(args)
			}
		}

		err := play.DoRequest(ctx, sess, playReq)

		result := &mcp.CallToolResult{}
		if err != nil {
			result.Content = []mcp.Content{
				&mcp.TextContent{Text: err.Error()},
			}
		} else if len(sess.Conn.Mcp.Data) > 0 {
			result.Content = []mcp.Content{
				&mcp.TextContent{Text: string(sess.Conn.Mcp.Data)},
			}
		}
		return result, nil
	}
}

func (i *mcpInstance) Run(listener net.Listener, udpListener net.PacketConn) error {
	switch i.transport {
	case play.MCP_TRANSPORT_STDIO:
		return i.mcpServer.Run(context.Background(), &mcp.StdioTransport{})
	case play.MCP_TRANSPORT_STREAMABLE_HTTP:
		i.httpHandler = mcp.NewStreamableHTTPHandler(
			func(r *http.Request) *mcp.Server { return i.mcpServer },
			nil,
		)
		return http.Serve(listener, i.httpHandler)
	default:
		return i.mcpServer.Run(context.Background(), &mcp.StdioTransport{})
	}
}

func (i *mcpInstance) AddActionUnits(units ...*play.ActionUnit) error {
	for _, u := range units {
		if i.actions[u.RequestName] != nil {
			return errors.New("action unit " + u.RequestName + " already exists in " + i.info.Name())
		}
		i.actions[u.RequestName] = u
		i.sortedNames = append(i.sortedNames, u.RequestName)
	}
	sort.Strings(i.sortedNames)
	return nil
}

func (i *mcpInstance) UpdateActionTimeout(spaceName string, actionName string, timeout time.Duration) {
	if spaceName != "" {
		spaceName = spaceName + "."
	}
	if act := i.actions[spaceName+actionName]; act != nil {
		act.Timeout = timeout
	}
}

// mcpPacker implements play.IPacker for MCP protocol
type mcpPacker struct{}

func (p *mcpPacker) Unpack(c *play.Conn) (*play.Request, error) {
	return nil, nil
}

func (p *mcpPacker) Pack(c *play.Conn, res *play.Response) ([]byte, error) {
	return json.Marshal(res.Output.All())
}

func actionFieldsToSchema(fields map[string]play.ActionField) *jsonschema.Schema {
	if len(fields) == 0 {
		return nil
	}
	properties := make(map[string]*jsonschema.Schema, len(fields))
	var required []string

	for _, field := range fields {
		key := field.Field
		if len(field.Keys) > 0 {
			key = field.Keys[0]
		}
		prop := &jsonschema.Schema{
			Type:        playTypeToSchemaType(field.Typ),
			Description: field.Desc,
		}
		if field.Child != nil {
			childSchema := actionFieldsToSchema(field.Child)
			if childSchema != nil {
				if field.Typ == "[]object" {
					prop.Items = childSchema
				} else {
					prop.Properties = childSchema.Properties
					prop.Required = childSchema.Required
				}
			}
		}
		properties[key] = prop
		if field.Required {
			required = append(required, key)
		}
	}
	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

func playTypeToSchemaType(typ string) string {
	switch typ {
	case "int", "int64", "uint", "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "[]object":
		return "array"
	case "object", "map":
		return "object"
	default:
		return "string"
	}
}
