package apijson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// oursExport is one corpus root exported through this repository's writer.
type oursExport struct {
	Elements []*element
	// Refused counts the files the parser could not read whole; a converted
	// graph would silently miss what the parser skipped, so they are left out
	// the way `sysml -convert` refuses them.
	Refused int
	// Separate reports that the root did not convert as one model and each
	// document was converted alone, so its cross-file references stay unresolved.
	Separate bool
	// Unconvertible counts the documents the writer refused when converted alone.
	Unconvertible int
	// Duplicates counts identical copies dropped; Ambiguous the elements two of
	// which computed the same key but differ; Shared names the first such keys.
	Duplicates int
	Ambiguous  int
	Shared     []string
	// Unkeyable counts the elements no key could address, Unkeyed per metaclass.
	Unkeyable int
	Unkeyed   map[string]int
}

// exportOurs converts the files of one root together, so cross-file imports
// resolve, and keys the elements the writer emits. When the root does not
// convert as one model (two documents declaring the same element, as the
// interchange fixtures do), each document is converted alone instead.
func exportOurs(dir string, files []string) (*oursExport, error) {
	result := &oursExport{Unkeyed: make(map[string]int)}
	documents := make([]export.ModelDocument, 0, len(files))
	for _, rel := range files {
		// #nosec G304 -- the corpus root to compare is named on the command line.
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		file := source.New(rel, content)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			result.Refused++
			continue
		}
		documents = append(documents, export.ModelDocument{File: file, Root: root})
	}
	if len(documents) == 0 {
		return result, nil
	}
	raw, err := convertOurs(documents)
	if err == nil {
		result.Elements, result.Unkeyable = keyOurs(raw, soleFile(documents), result.Unkeyed)
	} else {
		result.Separate = true
		for _, document := range documents {
			raw, err := convertOurs([]export.ModelDocument{document})
			if err != nil {
				result.Unconvertible++
				continue
			}
			elements, unkeyable := keyOurs(raw, document.File.Name(), result.Unkeyed)
			result.Elements = append(result.Elements, elements...)
			result.Unkeyable += unkeyable
		}
	}
	result.Elements, result.Duplicates, result.Ambiguous, result.Shared = dropAmbiguous(result.Elements)
	return result, nil
}

// soleFile names a root's one document, whose writer states no sysx:sourceDocument.
func soleFile(documents []export.ModelDocument) string {
	if len(documents) == 1 {
		return documents[0].File.Name()
	}
	return ""
}

// convertOurs writes the documents as one API-JSON array and decodes it.
func convertOurs(documents []export.ModelDocument) ([]map[string]any, error) {
	graph, err := export.ModelToRDFWith(documents, export.IDQualifiedName)
	if err != nil {
		return nil, err
	}
	encoded, err := export.WriteAPIJSON(graph)
	if err != nil {
		return nil, fmt.Errorf("write api-json: %w", err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return nil, fmt.Errorf("parse our api-json: %w", err)
	}
	return raw, nil
}

// ourKeyer computes stable keys over this repository's raw elements, memoized.
type ourKeyer struct {
	byID  map[string]map[string]any
	keys  idKeys
	files map[string]string
	// soleFile is the one document of a root exported alone, which states none.
	soleFile string
	// unkeyed counts the unkeyable elements per metaclass, for the report.
	unkeyed map[string]int
	// rootOrdinal counts the ownerless elements already keyed per (file, type).
	rootOrdinal map[string]int
	visiting    map[string]bool
}

// keyOurs keys the writer's elements the way the pilot bridge keys its own: a
// named element by its qualified name, an unnamed one from its owner, an
// ownerless one by its document and metaclass.
func keyOurs(raw []map[string]any, soleFile string, unkeyed map[string]int) ([]*element, int) {
	k := &ourKeyer{
		soleFile:    soleFile,
		unkeyed:     unkeyed,
		byID:        make(map[string]map[string]any, len(raw)),
		keys:        make(idKeys, len(raw)),
		files:       make(map[string]string, len(raw)),
		rootOrdinal: make(map[string]int),
		visiting:    make(map[string]bool),
	}
	for _, object := range raw {
		if id, _ := object["@id"].(string); id != "" {
			k.byID[id] = object
		}
	}
	var unkeyable int
	for _, object := range raw {
		id, _ := object["@id"].(string)
		if k.keyOf(id) == "" {
			unkeyable++
			typ, _ := object["@type"].(string)
			k.unkeyed[typ]++
		}
	}
	elements := make([]*element, 0, len(raw))
	for _, object := range raw {
		id, _ := object["@id"].(string)
		key := k.keys[id]
		if key == "" {
			continue
		}
		typ, _ := object["@type"].(string)
		e := &element{Key: key, Type: typ, File: k.fileOf(id), Props: make(map[string]any, len(object))}
		for property, value := range object {
			if property == "@id" || property == "@type" {
				continue
			}
			e.Props[property] = normalizeValue(property, value, k.keys)
		}
		elements = append(elements, e)
	}
	return elements, unkeyable
}

func (k *ourKeyer) keyOf(id string) string {
	if key, ok := k.keys[id]; ok {
		return key
	}
	object := k.byID[id]
	if object == nil || k.visiting[id] {
		return ""
	}
	k.visiting[id] = true
	defer delete(k.visiting, id)
	key := k.compute(id, object)
	k.keys[id] = key
	return key
}

func (k *ourKeyer) compute(id string, object map[string]any) string {
	if name, _ := object["qualifiedName"].(string); name != "" && !positional(name) {
		return canonicalKey(name)
	}
	typ, _ := object["@type"].(string)
	ownerID := refID(object["owner"])
	if ownerID == "" {
		file := k.fileOf(id)
		slot := file + "\x00" + typ
		ordinal := k.rootOrdinal[slot]
		k.rootOrdinal[slot]++
		return "/" + file + "/" + typ + "#" + fmt.Sprint(ordinal)
	}
	ownerKey := k.keyOf(ownerID)
	owner := k.byID[ownerID]
	if ownerKey == "" || owner == nil {
		return ""
	}
	var siblings []string
	if isRelationship(object) {
		siblings = refIDs(owner["ownedRelationship"])
	} else {
		siblings = k.ownedElements(owner)
	}
	index := 0
	for _, sibling := range siblings {
		if sibling == id {
			return ownerKey + "/" + typ + "[" + fmt.Sprint(index) + "]"
		}
		if other := k.byID[sibling]; other != nil {
			if otherType, _ := other["@type"].(string); otherType == typ {
				index++
			}
		}
	}
	return ""
}

// positional reports a qualified name the writer spells for an unnamed element
// from its position (Demo::@1, Legacy::@4::@trigger); the metamodel derives no
// qualifiedName for such an element, so it is keyed structurally like the pilot's.
func positional(name string) bool {
	for _, segment := range strings.Split(name, "::") {
		if strings.HasPrefix(segment, "@") {
			return true
		}
	}
	return false
}

// ownedElements lists an owner's owned elements in order: the stated
// ownedElement when the writer emits it, else each owned relationship's owned
// related elements in relationship order, which is how the metamodel derives it.
func (k *ourKeyer) ownedElements(owner map[string]any) []string {
	if stated := refIDs(owner["ownedElement"]); len(stated) > 0 {
		return stated
	}
	var owned []string
	for _, relationship := range refIDs(owner["ownedRelationship"]) {
		if object := k.byID[relationship]; object != nil {
			owned = append(owned, refIDs(object["ownedRelatedElement"])...)
		}
	}
	return owned
}

// fileOf is the document an element comes from: the one its root namespace
// states, found up the owner chain.
func (k *ourKeyer) fileOf(id string) string {
	if file, ok := k.files[id]; ok {
		return file
	}
	file := k.soleFile
	for current, depth := id, 0; current != "" && depth < 1<<16; depth++ {
		object := k.byID[current]
		if object == nil {
			break
		}
		if stated, _ := object["sysx:sourceDocument"].(string); stated != "" {
			file = stated
			break
		}
		current = refID(object["owner"])
	}
	k.files[id] = file
	return file
}

// isRelationship recognizes a relationship element by the properties only the
// Relationship metaclass carries.
func isRelationship(object map[string]any) bool {
	for _, property := range []string{"relatedElement", "owningRelatedElement", "ownedRelatedElement", "source", "target"} {
		if _, ok := object[property]; ok {
			return true
		}
	}
	return false
}
