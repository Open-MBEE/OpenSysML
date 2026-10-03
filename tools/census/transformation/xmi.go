package transformation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// node is one XML element of the transformation model, decoded generically:
// the mapping extraction needs only names, ids, types and parentage.
type node struct {
	tag      string // namespace URI
	local    string
	attrs    map[string]string // "<uri>|<local>" for qualified, "<local>" otherwise
	children []*node
	parent   *node
}

func (n *node) attr(local string) string {
	if v, ok := n.attrs[local]; ok {
		return v
	}
	for k, v := range n.attrs {
		if strings.HasSuffix(k, "|"+local) {
			return v
		}
	}
	return ""
}

func (n *node) xmiType() string { return n.attr("type") }
func (n *node) xmiID() string   { return n.attr("id") }
func (n *node) name() string    { return n.attr("name") }

func (n *node) is(kind string) bool { return n.xmiType() == kind }

// parseModel reads an XMI document into a generic element tree.
func parseModel(path string) (*node, error) {
	f, err := os.Open(path) // #nosec G304 -- callers name the located model file
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	var root *node
	stack := []*node{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{tag: t.Name.Space, local: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				if a.Name.Space != "" {
					n.attrs[a.Name.Space+"|"+a.Name.Local] = a.Value
				}
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				n.parent = stack[len(stack)-1]
				n.parent.children = append(n.parent.children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	if root == nil {
		return nil, fmt.Errorf("%s: empty document", path)
	}
	return root, nil
}

// mappingsPackage is the xmi:id of the model package the mapping classes live under.
const mappingsPackageID = "Mappings"

// extracted holds what one pass over the pinned XMI derives.
type extracted struct {
	Packages          int
	Classes           int
	OCLBodies         int
	OCLSpecifications int
	OCLPostconditions int
	OCLOwnedRules     int
	Mappings          []Mapping
}

// walk visits every element under n, n excluded.
func (n *node) walk(visit func(*node)) {
	for _, c := range n.children {
		visit(c)
		c.walk(visit)
	}
}

// extract reads the mapping classes of the pinned transformation model.
func extract(path string) (*extracted, error) {
	root, err := parseModel(path)
	if err != nil {
		return nil, err
	}
	var mappings *node
	for _, c := range root.children {
		for _, e := range c.children {
			if e.xmiID() == mappingsPackageID && e.is("uml:Package") {
				mappings = e
			}
		}
	}
	if mappings == nil {
		return nil, fmt.Errorf("%s: no package %q under the document root", path, mappingsPackageID)
	}
	out := &extracted{Packages: 1} // the Mappings package itself counts
	byID := map[string]*node{}
	mappings.walk(func(e *node) {
		if id := e.xmiID(); id != "" {
			byID[id] = e
		}
		if e.is("uml:Class") && (e.local == "packagedElement" || e.local == "nestedClassifier") {
			out.Classes++
		}
		if e.local == "packagedElement" && e.is("uml:Package") {
			out.Packages++
		}
	})
	mappings.walk(func(e *node) {
		if e.local == "specification" && e.attr("language") == "OCL2.0" {
			out.OCLSpecifications++
			switch e.parent.local {
			case "bodyCondition":
				out.OCLBodies++
			case "postcondition":
				out.OCLPostconditions++
			case "ownedRule":
				out.OCLOwnedRules++
			}
		}
	})
	m := &model{byID: byID}
	var list []Mapping
	seen := map[string]bool{}
	mappings.walk(func(e *node) {
		if !e.is("uml:Class") || (e.local != "packagedElement" && e.local != "nestedClassifier") {
			return
		}
		name := e.name()
		if !strings.HasSuffix(name, "_Mapping") {
			return
		}
		if seen[name] {
			m.dupes = append(m.dupes, name)
			return
		}
		seen[name] = true
		list = append(list, m.mapping(e))
	})
	if len(m.dupes) > 0 {
		return nil, fmt.Errorf("%s: mapping class names are not unique: %s", path, strings.Join(m.dupes, ", "))
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Package != list[j].Package {
			return list[i].Package < list[j].Package
		}
		return list[i].Name < list[j].Name
	})
	out.Mappings = list
	return out, nil
}

// model resolves cross-references inside the Mappings package.
type model struct {
	byID  map[string]*node
	dupes []string
}

// packagePath is the owning package's name path below the Mappings root; a
// class nested in another class takes its owner class's package.
func (m *model) packagePath(e *node) string {
	var parts []string
	for p := e.parent; p != nil && p.xmiID() != mappingsPackageID; p = p.parent {
		if p.is("uml:Package") && p.local == "packagedElement" {
			parts = append([]string{p.name()}, parts...)
		}
	}
	return strings.Join(parts, "::")
}

// ownerClass is the class a nestedClassifier is nested in, if any.
func (m *model) ownerClass(e *node) *node {
	for p := e.parent; p != nil; p = p.parent {
		if p.is("uml:Class") {
			return p
		}
		if p.local == "packagedElement" && p.is("uml:Package") {
			return nil
		}
	}
	return nil
}

// mapping derives one baseline row's extracted fields from a mapping class.
func (m *model) mapping(e *node) Mapping {
	pkg := m.packagePath(e)
	qualified := name(e)
	if owner := m.ownerClass(e); owner != nil {
		qualified = owner.name() + "::" + name(e)
	}
	if pkg != "" {
		qualified = pkg + "::" + qualified
	}
	row := Mapping{
		Name:      e.name(),
		Package:   pkg,
		Qualified: qualified,
		Abstract:  e.attr("isAbstract") == "true",
		From:      m.typedAttribute(e, "from"),
		To:        m.typedAttribute(e, "to"),
	}
	for _, g := range m.generals(e) {
		row.Generals = append(row.Generals, g.name())
	}
	var bodies int
	digest := sha256.New()
	for _, op := range e.children {
		if op.local != "ownedOperation" {
			continue
		}
		row.Operations = append(row.Operations, op.name())
		digest.Write([]byte(op.name()))
		digest.Write([]byte{0})
		for _, bc := range op.children {
			if bc.local != "bodyCondition" {
				continue
			}
			for _, s := range bc.children {
				if s.local == "specification" && s.attr("language") == "OCL2.0" {
					digest.Write([]byte(s.attr("body")))
					digest.Write([]byte{0})
					bodies++
				}
			}
		}
	}
	row.OCL = "sha256:" + hex.EncodeToString(digest.Sum(nil))
	return row
}

func name(e *node) string { return e.name() }

// generals are the class's direct generalization targets in document order.
func (m *model) generals(e *node) []*node {
	var out []*node
	for _, g := range e.children {
		if g.local != "generalization" {
			continue
		}
		for _, gg := range g.children {
			if gg.local == "general" {
				if ref := gg.attr("idref"); ref != "" {
					if target, ok := m.byID[ref]; ok {
						out = append(out, target)
					}
				}
			}
		}
	}
	return out
}

// typedAttribute resolves the named ownedAttribute's type: a class in the file
// by idref, or a metamodel element by href fragment (its last segment).
func (m *model) typedAttribute(e *node, name string) string {
	if ref := m.ownAttributeType(e, name); ref != "" {
		return ref
	}
	// No own attribute: breadth-first over generalizations in document order.
	seen := map[*node]bool{e: true}
	queue := m.generals(e)
	for len(queue) > 0 {
		g := queue[0]
		queue = queue[1:]
		if g == nil || seen[g] {
			continue
		}
		seen[g] = true
		if ref := m.ownAttributeType(g, name); ref != "" {
			return ref
		}
		queue = append(queue, m.generals(g)...)
	}
	return ""
}

// ownAttributeType is the class's own ownedAttribute's type name, "" when absent.
func (m *model) ownAttributeType(e *node, name string) string {
	for _, a := range e.children {
		if a.local != "ownedAttribute" || a.name() != name {
			continue
		}
		for _, ty := range a.children {
			if ty.local != "type" {
				continue
			}
			if ref := ty.attr("idref"); ref != "" {
				if target, ok := m.byID[ref]; ok {
					return target.name()
				}
				return ref
			}
			if href := ty.attr("href"); href != "" {
				frag := href[strings.LastIndex(href, "#")+1:]
				return frag[strings.LastIndex(frag, "-")+1:]
			}
		}
	}
	return ""
}
