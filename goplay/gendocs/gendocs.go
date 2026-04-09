package gendocs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/leochen2038/play/goplay/env"
)

// GenerateDocs 生成API文档的入口函数
func GenerateDocs() error {
	// 编译并运行项目来生成文档
	if err := buildAndRunForDocs(); err != nil {
		return fmt.Errorf("generate docs failed: %v", err)
	}
	fmt.Printf("API documentation generated successfully at: %s\n", env.ProjectPath)
	return nil
}

// buildAndRunForDocs 编译并运行项目以生成文档
func buildAndRunForDocs() error {
	tempBinary := filepath.Join(env.ProjectPath, "temp_for_docs")
	if runtime.GOOS == "windows" {
		tempBinary += ".exe"
	}

	// 确保在函数结束时删除临时文件
	defer os.Remove(tempBinary)

	// 编译项目
	if err := buildProject(tempBinary); err != nil {
		return err
	}

	// 运行项目生成文档
	if err := runProjectForDocs(tempBinary); err != nil {
		return err
	}

	return nil
}

// buildProject 编译项目
func buildProject(outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath)
	cmd.Dir = env.ProjectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build project failed: %v", err)
	}
	return nil
}

// runProjectForDocs 运行项目来生成文档
func runProjectForDocs(binaryPath string) error {
	docCmd := exec.Command(binaryPath)
	docCmd.Dir = env.ProjectPath
	docCmd.Env = []string{"GENDOC=true"}
	docCmd.Stdout = os.Stdout
	docCmd.Stderr = os.Stderr

	if err := docCmd.Run(); err != nil {
		return fmt.Errorf("run project for docs generation failed: %v", err)
	}
	return nil
}
