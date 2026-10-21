package libs

import (
	"fmt"
	"path"
)

// ModelFile is one bundled library file, named as the bundle lists it.
type ModelFile struct {
	Name string
	Data []byte
}

// BundledModelFiles returns every KerML and SysML file of the bundled standard
// library, in the bundle's order, as the files of one model: converted together
// they resolve each other's names, and each takes its bundled place, so the
// library's elements carry their normative ids.
func BundledModelFiles() ([]ModelFile, error) {
	bundled := BundledSource()
	names := bundled.List()
	files := make([]ModelFile, 0, len(names))
	for _, name := range names {
		switch path.Ext(name) {
		case ".kerml", ".sysml":
		default:
			continue
		}
		data, err := bundled.Read(name)
		if err != nil {
			return nil, fmt.Errorf("read the bundled library file %s: %w", name, err)
		}
		files = append(files, ModelFile{Name: name, Data: data})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("the bundled standard library holds no KerML or SysML file")
	}
	return files, nil
}
