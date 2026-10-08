// Package notebook reads the code cells of a Jupyter notebook (nbformat 4), so a
// model written in a notebook can be loaded where a model file can.
package notebook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ext is the file extension a notebook is named with.
const Ext = ".ipynb"

// SkipTag marks a code cell a load passes over: scratch work, a demonstration,
// anything the notebook runs that another should not inherit.
const SkipTag = "skip-load"

// Language is the kernelspec language a notebook must record to be loaded as a
// SysML model; a notebook recording none is taken to be one.
const Language = "sysml"

// IsNotebook reports whether path names a notebook, by its extension.
func IsNotebook(path string) bool {
	return strings.EqualFold(filepath.Ext(path), Ext)
}

// Cell is one code cell of a notebook.
type Cell struct {
	// Index is the cell's 1-based position among the notebook's code cells,
	// which is how a selection names it and how a diagnostic locates it.
	Index  int
	Source string
	Tags   []string
}

// Tagged reports whether the cell carries tag in its metadata.
func (c Cell) Tagged(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Notebook is what a load reads of a notebook: its code cells in notebook
// order, markdown and raw cells left out, and the language its kernel records.
type Notebook struct {
	Name     string
	Language string // kernelspec.language, else language_info.name; "" when neither is recorded
	Cells    []Cell
}

// Read reads the notebook at path. The error is a *FormatError for a file that
// is not an nbformat 4 notebook, or the read error.
func Read(path string) (*Notebook, error) {
	// #nosec G304 -- the file is one the user named, or one found under a
	// directory or pattern they named.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

// FormatError is a file that is not a notebook a load can read, and why.
type FormatError struct {
	Name   string
	Reason string
}

func (e *FormatError) Error() string {
	return fmt.Sprintf("cannot load %s: %s", e.Name, e.Reason)
}

// LanguageError is a notebook written for another kernel's language, which a
// load refuses rather than reading its cells as SysML.
type LanguageError struct {
	Name     string
	Language string
}

func (e *LanguageError) Error() string {
	return fmt.Sprintf("cannot load %s: a %s notebook; only a notebook whose kernel language is %s (or that records none) loads as a model",
		e.Name, e.Language, Language)
}

// rawNotebook is the nbformat JSON a load reads: the format version, the kernel
// language and the cells, each with its type, source and tags.
type rawNotebook struct {
	NBFormat *int `json:"nbformat"`
	Metadata struct {
		Kernelspec struct {
			Language string `json:"language"`
		} `json:"kernelspec"`
		LanguageInfo struct {
			Name string `json:"name"`
		} `json:"language_info"`
	} `json:"metadata"`
	Cells *[]struct {
		Type     string     `json:"cell_type"`
		Source   cellSource `json:"source"`
		Metadata struct {
			Tags []string `json:"tags"`
		} `json:"metadata"`
	} `json:"cells"`
}

// cellSource is a cell's text, which nbformat writes as one string or as a
// list of lines that keep their newlines.
type cellSource string

func (s *cellSource) UnmarshalJSON(data []byte) error {
	var whole string
	if err := json.Unmarshal(data, &whole); err == nil {
		*s = cellSource(whole)
		return nil
	}
	var lines []string
	if err := json.Unmarshal(data, &lines); err != nil {
		return errors.New("a cell's source must be a string or a list of strings")
	}
	*s = cellSource(strings.Join(lines, ""))
	return nil
}

// Parse reads data as the nbformat 4 notebook called name. A notebook of
// another format version, or a file that is not a notebook at all, is a
// *FormatError; one written for another language is a *LanguageError.
func Parse(name string, data []byte) (*Notebook, error) {
	var raw rawNotebook
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return nil, &FormatError{Name: name, Reason: "not a notebook: " + jsonReason(err)}
	}
	if raw.NBFormat == nil {
		return nil, &FormatError{Name: name, Reason: "not a notebook: no nbformat version"}
	}
	if *raw.NBFormat != 4 {
		return nil, &FormatError{Name: name, Reason: fmt.Sprintf("nbformat %d is not read; save the notebook as nbformat 4", *raw.NBFormat)}
	}
	if raw.Cells == nil {
		return nil, &FormatError{Name: name, Reason: "not a notebook: no cells"}
	}
	nb := &Notebook{Name: name, Language: raw.Metadata.Kernelspec.Language}
	if nb.Language == "" {
		nb.Language = raw.Metadata.LanguageInfo.Name
	}
	if nb.Language != "" && !strings.EqualFold(nb.Language, Language) {
		return nil, &LanguageError{Name: name, Language: nb.Language}
	}
	for _, cell := range *raw.Cells {
		if cell.Type != "code" {
			continue
		}
		nb.Cells = append(nb.Cells, Cell{
			Index:  len(nb.Cells) + 1,
			Source: string(cell.Source),
			Tags:   cell.Metadata.Tags,
		})
	}
	return nb, nil
}

// jsonReason is a JSON error as a reader of the notebook needs it, without the
// Go type names the decoder puts in it.
func jsonReason(err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("invalid JSON at byte %d: %s", syntax.Offset, syntax.Error())
	}
	var kind *json.UnmarshalTypeError
	if errors.As(err, &kind) {
		return fmt.Sprintf("%s is not %s", kind.Field, kind.Value)
	}
	return err.Error()
}
