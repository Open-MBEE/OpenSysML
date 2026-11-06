package export

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// libraryDocumentForm is the element form of one library document, converted
// once per library index: the library is immutable once loaded, and the whole
// of it is far too large to convert for one element.
type libraryDocumentForm struct {
	once sync.Once
	form *ElementForm
	err  error
}

// LibraryElementForm reads a standard-library element by the id a model's form
// references it under, or by its qualified name: the library document declaring
// it is converted as a model of its own — which gives its elements their
// normative ids, as -stdlib conversion does — and the element is read from that
// form, which is also returned so the element's references can be named. The
// document is the text of the library the shared index was built from, the
// configured one (LibraryPathEnvVar) included. False when no library element
// is known by idOrName.
func LibraryElementForm(idOrName string) (APIJSONElement, *ElementForm, bool, error) {
	index := sharedLibraryIndex()
	doc, id, ok := index.elementDocument(idOrName)
	if !ok {
		return APIJSONElement{}, nil, false, nil
	}
	form, err := index.documentElementForm(doc)
	if err != nil {
		return APIJSONElement{}, nil, true, err
	}
	element, found := form.ByID(id)
	if !found {
		return APIJSONElement{}, nil, true, fmt.Errorf("library document %s writes no element %s", doc, id)
	}
	return element, form, true, nil
}

// elementDocument names the library document declaring the element idOrName
// refers to, and the element's id: a catalogued element by id or qualified
// name, its owning membership, or an unnamed member by minted id.
func (index *libraryIndex) elementDocument(idOrName string) (doc, id string, ok bool) {
	if element, found := index.catalog.Element(idOrName); found {
		return element.Symbol.DocName, element.ID, true
	}
	if element, found := index.catalog.ElementNamed(idOrName); found {
		return element.Symbol.DocName, element.ID, true
	}
	if element, found := index.catalog.OwningMembership(idOrName); found {
		return element.Symbol.DocName, element.OwningMembershipID, true
	}
	if member, found := index.anonymous.Element(idOrName); found && member.Owner != nil {
		return member.Owner.Symbol.DocName, member.ID, true
	}
	if member, found := index.anonymous.OwningMembership(idOrName); found && member.Owner != nil {
		return member.Owner.Symbol.DocName, member.OwningMembershipID, true
	}
	return "", "", false
}

// documentElementForm converts one document of the index's library on first
// use, from the text the index was built from. A library document among a
// model's documents takes the library's own copy's place (analyzeModel), so
// its elements carry the normative ids and resolve the rest of the library as
// that copy does.
func (index *libraryIndex) documentElementForm(doc string) (*ElementForm, error) {
	entry, _ := index.forms.LoadOrStore(doc, &libraryDocumentForm{})
	cached := entry.(*libraryDocumentForm)
	cached.once.Do(func() {
		data, err := index.source.Read(doc)
		if err != nil {
			cached.err = fmt.Errorf("read the library document %s: %w", doc, err)
			return
		}
		file := source.New(doc, data)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			cached.err = fmt.Errorf("library document %s: %d syntax error(s)", doc, len(p.Diagnostics))
			return
		}
		cached.form, cached.err = modelElementFormWith([]ModelDocument{{File: file, Root: root}}, IDUUID)
	})
	return cached.form, cached.err
}
