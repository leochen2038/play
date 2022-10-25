package packers

import (
	"encoding/binary"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/codec/binders"
	"github.com/leochen2038/play/codec/renders"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type PbPacker struct {
	fileDescriptors []protoreflect.FileDescriptor
}

func NewPbPacker(fileDescriptors []protoreflect.FileDescriptor) play.IPacker {
	return &PbPacker{fileDescriptors: fileDescriptors}
}

func (p *PbPacker) Receive(c *play.Conn) (*play.Request, error) {
	var err error
	var request play.Request
	request.RenderName = "pb"

	switch c.Type {
	case play.SERVER_TYPE_HTTP, play.SERVER_TYPE_H2C:
		request.ActionName = ParseHttp2Path(c.Http.Request.URL.Path)
		request.InputBinder, err = getBinderOfProtobuf(c.Http.Request, p.fileDescriptors)
	default:
		return nil, errors.New("json packer not support " + strconv.Itoa(c.Type) + " type")
	}

	return &request, err
}

func (p *PbPacker) Pack(c *play.Conn, res *play.Response) ([]byte, error) {
	w := c.Http.ResponseWriter
	w.Header().Set("content-type", "application/grpc")
	w.Header().Set("status", "200")
	w.Header().Set("trailer", "grpc-status, grpc-message")
	w.Header().Set("grpc-status", "0")
	w.Header().Set("grpc-message", "ok")

	descriptor := getMessageDescriptor(c.Http.Request, p.fileDescriptors, false)
	if descriptor == nil {
		return nil, errors.New("descriptor not found")
	}

	data, err := renders.GetRenderOfProtobuf(descriptor).Render(res.Output.All())
	if err != nil {
		return nil, err
	}

	head := make([]byte, 5)
	binary.BigEndian.PutUint32(head[1:5], uint32(len(data)))
	data = append(head, data...)

	return data, nil
}

func ParseHttp2Path(path string) string {
	return strings.ReplaceAll(path[1:], "/", ".")
}

func getBinderOfProtobuf(request *http.Request, fileDescriptors []protoreflect.FileDescriptor) (binders.Binder, error) {
	descriptor := getMessageDescriptor(request, fileDescriptors, true)
	if descriptor == nil {
		return nil, errors.New("descriptor not found")
	}
	msg := dynamicpb.NewMessage(descriptor)

	data, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, errors.New("read grpc request message err:" + err.Error())
	}

	err = proto.Unmarshal(data[5:], msg)
	if err != nil {
		return nil, errors.New("unmarshal grpc request message err:" + err.Error())
	}
	return binders.GetBinderOfProtobuf(msg.ProtoReflect()), nil
}

func getMessageDescriptor(request *http.Request, fileDescriptors []protoreflect.FileDescriptor, isRequest bool) protoreflect.MessageDescriptor {
	packName, requestName, responseName := parseHttp2Path(request)
	var fileDescriptor protoreflect.FileDescriptor
	for _, descriptor := range fileDescriptors {
		if packName == string(descriptor.Name()) {
			fileDescriptor = descriptor
		}
	}
	if fileDescriptor == nil {
		return nil
	}
	if isRequest {
		return fileDescriptor.Messages().ByName(protoreflect.Name(requestName))
	}
	return fileDescriptor.Messages().ByName(protoreflect.Name(responseName))
}

func parseHttp2Path(request *http.Request) (string, string, string) {
	requestPath := strings.Split(request.URL.Path, "/")
	requestName := requestPath[len(requestPath)-1] + "Request"
	responseName := requestPath[len(requestPath)-1] + "Response"
	requestPath = strings.Split(request.URL.Path, ".")
	packName := strings.TrimLeft(requestPath[0], "/")
	return packName, requestName, responseName
}
