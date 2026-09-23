package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// IDForm is how the encoder spells the derived ids of elements, memberships
// and expression nodes that no ElementId annotation declares.
type IDForm int

const (
	// IDQualifiedName derives each id from the element's qualified name, so an
	// id reads where its element sits in the model. The default.
	IDQualifiedName IDForm = iota
	// IDUUID derives each id as a name-based uuid the way the SysML v2 library
	// convention does: the model's package namespace is
	// uuid5(NamespaceURL, elementIRI(root)), and every derived id under it is
	// uuid5(pkg, the qualified-name-derived local id it otherwise carries).
	IDUUID
)

// ParseIDForm reads the -id flag's argument.
func ParseIDForm(s string) (IDForm, bool) {
	switch s {
	case "qualified", "":
		return IDQualifiedName, true
	case "uuid":
		return IDUUID, true
	}
	return IDQualifiedName, false
}

// pkgFor returns the uuid namespace the root element IRI roots the uuid id
// form under — a scoped IRI where the document carries more than one scope,
// so same-named roots of different scopes derive different namespaces.
func (f *identityFacts) pkgFor(rootIRI string) string {
	if pkg, ok := f.pkg[rootIRI]; ok {
		return pkg
	}
	pkg := identity.NamespaceOf(rootIRI)
	f.pkg[rootIRI] = pkg
	return pkg
}

// minted applies the id form to a derived subject built from the parent's IRI
// plus suffix — an owning membership or materialized relationship — keeping
// the qualified form's term and minting uuid5(pkg, local) in uuid form.
func (f *identityFacts) minted(term, parent rdf.Term, suffix string) rdf.Term {
	if f.form != IDUUID {
		return term
	}
	pkg := f.pkgOf[parent.Value]
	if pkg == "" {
		return term
	}
	local := f.localOf[parent.Value] + suffix
	out := rdf.ElementIRIForID(identity.DerivedID(pkg, local))
	f.localOf[out.Value] = local
	f.pkgOf[out.Value] = pkg
	return out
}

// mintedNode applies the id form to a derived expression node under parent,
// whose node-local id is the parent's local id extended by path.
func (f *identityFacts) mintedNode(term, parent rdf.Term, path string) rdf.Term {
	if f.form != IDUUID {
		return term
	}
	pkg := f.pkgOf[parent.Value]
	if pkg == "" {
		return term
	}
	local := rdf.ExpressionNodeID(f.localOf[parent.Value], path)
	out := rdf.IRI(rdf.Expression + identity.DerivedID(pkg, local))
	f.localOf[out.Value] = local
	f.pkgOf[out.Value] = pkg
	return out
}

// record notes a subject's namespace uuid and name-derived local id for the
// derived ids minted under it.
func (f *identityFacts) record(subject rdf.Term, pkg, local string) {
	f.pkgOf[subject.Value] = pkg
	f.localOf[subject.Value] = local
}

// rootOf returns the qualified name of the document root qualifiedName sits
// under — the first of its segments.
func rootOf(qualifiedName string) string {
	if i := strings.Index(qualifiedName, "::"); i >= 0 {
		return qualifiedName[:i]
	}
	return qualifiedName
}
