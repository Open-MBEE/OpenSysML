package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Init writes the .project.json and .meta.json of a new project in dir,
// which must hold neither yet; the metadata indexes the model files already
// there. Name is required; publisher may be empty.
func Init(dir, publisher, name, version string) (*Interchange, error) {
	if name == "" {
		return nil, fmt.Errorf("%s: a project needs a name", dir)
	}
	if version == "" {
		version = "0.1.0"
	}
	if _, err := canonicalVersion(version); err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	for _, file := range []string{ProjectFile, MetaFile} {
		if fileExists(filepath.Join(dir, file)) {
			return nil, fmt.Errorf("%s already holds %s", dir, file)
		}
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	raw := map[string]any{"name": name, "version": version, "usage": []any{}}
	if publisher != "" {
		raw["publisher"] = publisher
	}
	content, err := marshalSorted(raw)
	if err != nil {
		return nil, err
	}
	if err := writeNew(filepath.Join(dir, ProjectFile), content); err != nil {
		return nil, err
	}
	members, err := bundleMembers(dir)
	if err != nil {
		return nil, err
	}
	meta, err := generateMeta(members)
	if err != nil {
		_ = os.Remove(filepath.Join(dir, ProjectFile))
		return nil, err
	}
	if err := writeNew(filepath.Join(dir, MetaFile), meta); err != nil {
		return nil, err
	}
	return ReadInterchange(dir)
}

// marshalSorted writes a JSON object with sorted keys, a two-space indent
// and a trailing newline.
func marshalSorted(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
