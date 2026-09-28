package fmi

import (
	"archive/zip"
	"io"
	"os"
	"sort"
	"strings"
)

// Read opens the .fmu archive at path and reads its model description and the
// platforms its binaries are built for.
func Read(path string) (*Description, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, ErrNotFMU
	}
	defer archive.Close() // the entries below are read before the archive is released
	return describeArchive(&archive.Reader)
}

// ReadArchive reads the .fmu archive held in memory at r.
func ReadArchive(r io.ReaderAt, size int64) (*Description, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return nil, ErrNotFMU
	}
	return describeArchive(archive)
}

// describeArchive reads the modelDescription.xml at the archive's root and
// lists the platforms binaries/<platform>/ holds files for.
func describeArchive(archive *zip.Reader) (*Description, error) {
	platforms := make(map[string]bool)
	var description []byte
	for _, f := range archive.File {
		name := f.Name
		if name == modelDescriptionName {
			rc, err := f.Open()
			if err != nil {
				return nil, &ModelDescriptionError{Detail: "modelDescription.xml cannot be read", Err: err}
			}
			data, err := drainClose(rc)
			if err != nil {
				return nil, &ModelDescriptionError{Detail: "modelDescription.xml cannot be read", Err: err}
			}
			description = data
			continue
		}
		if rest, ok := strings.CutPrefix(name, "binaries/"); ok && !strings.HasSuffix(name, "/") {
			if slash := strings.IndexByte(rest, '/'); slash > 0 {
				platforms[rest[:slash]] = true
			}
		}
	}
	if description == nil {
		return nil, ErrNotFMU
	}
	d, err := ParseModelDescription(description)
	if err != nil {
		return nil, err
	}
	for p := range platforms {
		d.Platforms = append(d.Platforms, p)
	}
	sort.Strings(d.Platforms)
	return d, nil
}
