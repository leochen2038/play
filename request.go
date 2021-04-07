package play

import "github.com/leochen2038/play/parsers"

type Request struct {
	Version  byte
	Format string
	Render string
	Caller string
	Tag    string
	TraceId  string
	SpanId   []byte
	Respond  bool
	ActionName   string
	Parser parsers.Parser
}
