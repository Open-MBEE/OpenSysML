package export

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// libraryDocumentForms holds the element form of each bundled library document
// a reader asked for, converted once per process: the library is immutable, and
// the whole of it is far too large to convert for one element.
var libraryDocumentForms sync.Map // document name -> *libraryDocumentForm

type libraryDocumentForm struct {
	once sync.Once
	form *ElementForm
	err  error
}

// LibraryElementForm reads a standard-library element by the id a model's form
// references it under, or by its qualified name: the bundled document declaring
// it is converted as a model of its own — which gives its elements their
// normative ids, as -stdlib conversion does — and the element is read from that
// form, which is also returned so the element's references can be named.
// False when no library element is known by idOrName.
func LibraryElementForm(idOrName string) (APIJSONElement, *ElementForm, bool, error) {
	doc, id, ok := libraryElementDocument(idOrName)
	if !ok {
		return APIJSONElement{}, nil, false, nil
	}
	form, err := libraryDocumentElementForm(doc)
	if err != nil {
		return APIJSONElement{}, nil, true, err
	}
	element, found := form.ByID(id)
	if !found {
		return APIJSONElement{}, nil, true, fmt.Errorf("library document %s writes no element %s", doc, id)
	}
	return element, form, true, nil
}

// libraryElementDocument names the bundled document declaring the library
// element idOrName refers to, and the element's id: a catalogued element by id
// or qualified name, its owning membership, or an unnamed member by minted id.
func libraryElementDocument(idOrName string) (doc, id string, ok bool) {
	index := sharedLibraryIndex()
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

// libraryDocumentElementForm converts one bundled library document on first
// use. A library document among a model's documents takes the bundled one's
// place (analyzeModel), so its elements carry the normative ids and resolve
// the rest of the library as the bundled copy does.
func libraryDocumentElementForm(doc string) (*ElementForm, error) {
	entry, _ := libraryDocumentForms.LoadOrStore(doc, &libraryDocumentForm{})
	cached := entry.(*libraryDocumentForm)
	cached.once.Do(func() {
		data, err := libs.BundledSource().Read(doc)
		if err != nil {
			cached.err = fmt.Errorf("read the bundled library document %s: %w", doc, err)
			return
		}
		file := source.New(doc, data)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			cached.err = fmt.Errorf("bundled library document %s: %d syntax error(s)", doc, len(p.Diagnostics))
			return
		}
		cached.form, cached.err = modelElementFormWith([]ModelDocument{{File: file, Root: root}}, IDUUID)
	})
	return cached.form, cached.err
}
