package exec

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type execCase struct {
	ID         string
	Target     string
	Expression string
}

type execModel struct {
	Path string
	Line int
}

type execCaseFile struct {
	Path   string
	Models []execModel
	Cases  []execCase
}

func readCaseFiles(dir string) ([]execCaseFile, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.cases"))
	if err != nil {
		return nil, fmt.Errorf("find case files: %w", err)
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no .cases files found in %s", dir)
	}
	files := make([]execCaseFile, 0, len(matches))
	for _, path := range matches {
		file, err := readCaseFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func readCaseFile(path string) (execCaseFile, error) {
	// #nosec G304 -- case paths come from the selected cases directory.
	f, err := os.Open(path)
	if err != nil {
		return execCaseFile{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	file := execCaseFile{Path: path}
	scanner := bufio.NewScanner(f)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "model:") {
			model := strings.TrimSpace(strings.TrimPrefix(line, "model:"))
			if model == "" {
				return execCaseFile{}, fmt.Errorf("%s:%d: model path is empty", path, lineNumber)
			}
			file.Models = append(file.Models, execModel{Path: model, Line: lineNumber})
			continue
		}
		fields := strings.SplitN(line, " :: ", 3)
		if len(fields) != 3 {
			return execCaseFile{}, fmt.Errorf("%s:%d: expected id :: target :: expression", path, lineNumber)
		}
		id, target, expression := strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1]), strings.TrimSpace(fields[2])
		if id == "" {
			return execCaseFile{}, fmt.Errorf("%s:%d: case id is empty", path, lineNumber)
		}
		if expression == "" {
			return execCaseFile{}, fmt.Errorf("%s:%d: expression is empty", path, lineNumber)
		}
		file.Cases = append(file.Cases, execCase{ID: id, Target: target, Expression: expression})
	}
	if err := scanner.Err(); err != nil {
		return execCaseFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(file.Cases) == 0 {
		return execCaseFile{}, fmt.Errorf("%s: no evaluation cases", path)
	}
	return file, nil
}
