package action

import (
	"fmt"
	"time"
)

func getProcessorTpl(pacekageNme, frameworkName, processorName string) string {
	return fmt.Sprintf(`package %s

import (
	"%s"
)

// copyright 2019-2020 The Play Framework
//
// %s.
// version 1.0.0
// author anonymous <anonymous@example.com>
// copyright 2019-2020 The Play Framework
//
// history:
// 1.0.0 | anonymous | %s | initialization

type %s struct {
	Input struct {}
	Output struct {}
}

func (p *%s)Run(ctx *play.Context) (string, error) {
	// TODO
	return "RC_NORMAL", nil
}
`, pacekageNme, frameworkName, processorName, time.Now().Format("2006-01-02 15:04:05"), processorName, processorName)
}
