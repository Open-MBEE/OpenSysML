package main

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"sort"
)

// validatorClasses are the two pilot validator classes whose string constants
// name the constraints, keyed by the language each one validates.
var validatorClasses = []struct {
	Language string
	Path     string
}{
	{Language: "kerml", Path: "org/omg/kerml/xtext/validation/KerMLValidator.class"},
	{Language: "sysml", Path: "org/omg/sysml/xtext/validation/SysMLValidator.class"},
}

// constraintIdentifierPattern selects the constraint identifiers among the
// classes' string constants and captures the specification's name inside them:
// the pilot compiles some with a trailing underscore (`validateUsageType_`) and
// one with a stray prefix (`invalidateMetadataFeatureBody`).
var constraintIdentifierPattern = regexp.MustCompile(`^(?:in)?(validate[A-Za-z]+?)_?$`)

// constraintNamePattern is the shape of a normalized name.
var constraintNamePattern = regexp.MustCompile(`^validate[A-Za-z]+$`)

// extractionMethod is the sentence the baseline records so a reader can repeat
// the extraction without this program.
const extractionMethod = "CONSTANT_String entries of each class's constant pool matching ^(in)?validate[A-Za-z]+_?$, normalized to the validate... name by dropping the in prefix and one trailing underscore; a name found in both classes has source \"both\""

// Extracted is one constraint name as read from the jar.
type Extracted struct {
	Name string
	// Raw is the identifier as compiled, kept when it differs from Name.
	Raw    string
	Source string
}

// extractFromJar reads the constraint identifiers out of the pinned jar's two
// validator classes and merges them into one sorted list.
func extractFromJar(path string) ([]Extracted, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open jar %s: %w", path, err)
	}
	defer func() { _ = reader.Close() }()

	byName := make(map[string]*Extracted)
	for _, class := range validatorClasses {
		content, err := readZipEntry(&reader.Reader, class.Path)
		if err != nil {
			return nil, err
		}
		constants, err := stringConstants(content)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", class.Path, err)
		}
		for _, raw := range constants {
			match := constraintIdentifierPattern.FindStringSubmatch(raw)
			if match == nil {
				continue
			}
			name := match[1]
			entry, seen := byName[name]
			if !seen {
				entry = &Extracted{Name: name, Source: class.Language}
				byName[name] = entry
			} else if entry.Source != class.Language {
				entry.Source = "both"
			}
			if raw != name {
				entry.Raw = raw
			}
		}
	}
	out := make([]Extracted, 0, len(byName))
	for _, entry := range byName {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func readZipEntry(reader *zip.Reader, name string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		defer func() { _ = rc.Close() }()
		content, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		return content, nil
	}
	return nil, fmt.Errorf("jar has no entry %s", name)
}

// stringConstants returns the CONSTANT_String values of a class file's constant
// pool in pool order (JVMS §4.4).
func stringConstants(class []byte) ([]string, error) {
	const header = 10 // magic, minor, major, constant_pool_count
	if len(class) < header || binary.BigEndian.Uint32(class) != 0xCAFEBABE {
		return nil, fmt.Errorf("not a class file")
	}
	count := int(binary.BigEndian.Uint16(class[8:]))
	utf8 := make(map[int]string, count)
	var stringRefs []int
	pos := header
	need := func(n int) error {
		if pos+n > len(class) {
			return fmt.Errorf("constant pool truncated at byte %d", pos)
		}
		return nil
	}
	for index := 1; index < count; index++ {
		if err := need(1); err != nil {
			return nil, err
		}
		tag := class[pos]
		pos++
		switch tag {
		case 1: // Utf8
			if err := need(2); err != nil {
				return nil, err
			}
			length := int(binary.BigEndian.Uint16(class[pos:]))
			pos += 2
			if err := need(length); err != nil {
				return nil, err
			}
			utf8[index] = string(class[pos : pos+length])
			pos += length
		case 3, 4: // Integer, Float
			pos += 4
		case 5, 6: // Long, Double take two slots
			pos += 8
			index++
		case 7, 16, 19, 20: // Class, MethodType, Module, Package
			pos += 2
		case 8: // String
			if err := need(2); err != nil {
				return nil, err
			}
			stringRefs = append(stringRefs, int(binary.BigEndian.Uint16(class[pos:])))
			pos += 2
		case 9, 10, 11, 12, 17, 18: // refs, NameAndType, Dynamic, InvokeDynamic
			pos += 4
		case 15: // MethodHandle
			pos += 3
		default:
			return nil, fmt.Errorf("constant pool entry %d has unknown tag %d", index, tag)
		}
		if pos > len(class) {
			return nil, fmt.Errorf("constant pool truncated at byte %d", pos)
		}
	}
	out := make([]string, 0, len(stringRefs))
	for _, ref := range stringRefs {
		value, ok := utf8[ref]
		if !ok {
			return nil, fmt.Errorf("CONSTANT_String refers to non-Utf8 entry %d", ref)
		}
		out = append(out, value)
	}
	return out, nil
}
