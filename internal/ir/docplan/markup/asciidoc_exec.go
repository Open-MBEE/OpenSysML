//go:build !js && !wasip1

package markup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// locateAsciiDoc finds the converter: the executable the variable names, else
// the tool on PATH.
func locateAsciiDoc() (string, error) {
	name := strings.TrimSpace(os.Getenv(AsciiDocEnv))
	if name == "" {
		name = asciiDocTool
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", &ToolError{Tool: name, EnvVar: AsciiDocEnv, Missing: true}
	}
	if !filepath.IsAbs(path) {
		if path, err = filepath.Abs(path); err != nil {
			return "", &ToolError{Tool: name, EnvVar: AsciiDocEnv, Missing: true, Detail: err.Error()}
		}
	}
	return path, nil
}

// runAsciiDoc feeds the body to the converter and returns its JSON.
func runAsciiDoc(body string) ([]byte, error) {
	path, err := locateAsciiDoc()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), asciiDocTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path) // #nosec G204 -- the operator names the converter
	cmd.Stdin = strings.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			detail = fmt.Sprintf("no answer within %s", asciiDocTimeout)
		}
		return nil, &ToolError{Tool: path, EnvVar: AsciiDocEnv, Detail: detail}
	}
	return stdout.Bytes(), nil
}
