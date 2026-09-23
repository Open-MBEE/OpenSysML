package migrate

import (
	"strings"
	"testing"
)

// useCaseModel wraps members beside the actor User (_user), the block Shop
// (_shop) and the use case Browse (_browse); stereotype is applied after the model.
func useCaseModel(members, stereotype string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Custom="http://www.example.org/profiles/Custom">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Actor" xmi:id="_user" name="User"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_shop" name="Shop"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_browse" name="Browse"/>
    ` + members + `
  </uml:Model>
  <sysml:Block xmi:id="_sb" base_Class="_shop"/>
  ` + stereotype + `
</xmi:XMI>`
}

func TestUseCaseForms(t *testing.T) {
	for _, tc := range []struct {
		name, members, stereotype string
		want                      []string
		id                        string
		verdict                   Verdict
	}{
		{"a use case with a subject is a use case def",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy" subject="_shop"/>`, "",
			[]string{"use case def Buy {\n    subject shop : Shop;\n}"}, "_uc", Mapped},
		{"an incidental stereotype does not change the form",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy"/>`,
			`<Custom:HyperlinkOwner xmi:id="_st" base_Element="_uc"/>`,
			[]string{"use case def Buy {\n    /* applied stereotype «HyperlinkOwner» */\n}"}, "_uc", Mapped},
		{"a second subject is a reference usage",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy" subject="_shop _user"/>`, "",
			[]string{"subject shop : Shop;\n    ref part user : User;"}, "_uc", Approximated},
		{"subjects written as usages are subset, not typed",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Review" subject="_ov _sv"/>
			 <packagedElement xmi:type="uml:Class" xmi:id="_ov" name="Overview"/>
			 <packagedElement xmi:type="uml:Class" xmi:id="_sv" name="Safety"/>`,
			`<sysml:View xmi:id="_st1" base_Class="_ov"/><sysml:Viewpoint xmi:id="_st2" base_Class="_sv"/>`,
			[]string{"subject overview :> Overview;\n    ref viewpoint safety :> Safety;"}, "_uc", Approximated},
		{"a subject that is a nested definition is named by its qualified name",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Review" subject="_sys"/>
			 <packagedElement xmi:type="uml:Class" xmi:id="_rep" name="Report">
			   <nestedClassifier xmi:type="uml:Class" xmi:id="_sys" name="System"/>
			 </packagedElement>`,
			`<sysml:Block xmi:id="_st1" base_Class="_rep"/><sysml:Block xmi:id="_st2" base_Class="_sys"/>`,
			[]string{"use case def Review {\n    subject system : Report::System;\n}"}, "_uc", Mapped},
		{"an association-owned actor end is an actor of the use case",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy"/>
			 <packagedElement xmi:type="uml:Association" xmi:id="_a" memberEnd="_e1 _e2">
			   <ownedEnd xmi:type="uml:Property" xmi:id="_e1" type="_user" association="_a"/>
			   <ownedEnd xmi:type="uml:Property" xmi:id="_e2" type="_uc" association="_a"/>
			 </packagedElement>`, "",
			[]string{"use case def Buy {\n    subject;\n    actor user : User;\n}"}, "_a", Mapped},
		{"an actor end the use case owns is written once, as its actor",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="customer" type="_user" association="_a"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:Association" xmi:id="_a" memberEnd="_p _e2">
			   <ownedEnd xmi:type="uml:Property" xmi:id="_e2" type="_uc" association="_a"/>
			 </packagedElement>`, "",
			[]string{"use case def Buy {\n    subject;\n    actor customer : User;\n}"}, "_p", Mapped},
		{"an include is an include use case usage",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
			   <include xmi:type="uml:Include" xmi:id="_inc" includingCase="_uc" addition="_browse"/>
			 </packagedElement>`, "",
			[]string{"include use case browse : Browse;"}, "_inc", Mapped},
		{"an include of a use case outside the document is refused",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
			   <include xmi:type="uml:Include" xmi:id="_inc" includingCase="_uc" addition="_gone"/>
			 </packagedElement>`, "",
			[]string{"not migrated: Include (_inc) — the included use case is not in the document"}, "_inc", Unmapped},
		{"an extend is a dependency keeping its point and condition",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
			   <extend xmi:type="uml:Extend" xmi:id="_ext" extension="_uc" extendedCase="_browse" extensionLocation="_ep">
			     <condition xmi:type="uml:Constraint" xmi:id="_cond">
			       <specification xmi:type="uml:LiteralBoolean" xmi:id="_spec" value="true"/>
			     </condition>
			   </extend>
			 </packagedElement>
			 <packagedElement xmi:type="uml:UseCase" xmi:id="_ext2" name="Search">
			   <extensionPoint xmi:type="uml:ExtensionPoint" xmi:id="_ep" name="found"/>
			 </packagedElement>`, "",
			[]string{"dependency Buy to Browse; /* extends at extension point(s) 'found' when true */",
				"not migrated: ExtensionPoint 'found'"}, "_ext", Approximated},
		{"an extend keeps an extension point that is not in the document",
			`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
			   <extend xmi:type="uml:Extend" xmi:id="_ext" extension="_uc" extendedCase="_browse" extensionLocation="_gone"/>
			 </packagedElement>`, "",
			[]string{"dependency Buy to Browse; /* extends at extension point(s) _gone (not in the document) */"}, "_ext", Approximated},
		{"a block property typed by a use case is a reference use case usage",
			`<packagedElement xmi:type="uml:Class" xmi:id="_c" name="Site">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="visit" type="_browse"/>
			 </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_c"/>`,
			[]string{"part def Site {\n    ref use case visit : Browse;\n}"}, "_p", Approximated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("cases.xmi", []byte(useCaseModel(tc.members, tc.stereotype)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			var found bool
			for _, e := range r.Report.Entries {
				if e.ID != tc.id {
					continue
				}
				found = true
				if e.Verdict != tc.verdict {
					t.Errorf("verdict %s, want %s (%s)", e.Verdict, tc.verdict, e.Note)
				}
			}
			if !found {
				t.Errorf("%s is missing from the report", tc.id)
			}
		})
	}
}

// TestUseCaseSubjectFeaturedElsewhere names as subject a view nested in a block:
// a feature only the block's members can name, so no subject is written and
// the report says why.
func TestUseCaseSubjectFeaturedElsewhere(t *testing.T) {
	r, err := Migrate("cases.xmi", []byte(useCaseModel(
		`<packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Review" subject="_ov"/>
		 <packagedElement xmi:type="uml:Class" xmi:id="_rep" name="Report">
		   <nestedClassifier xmi:type="uml:Class" xmi:id="_ov" name="Overview"/>
		 </packagedElement>`,
		`<sysml:Block xmi:id="_st1" base_Class="_rep"/><sysml:View xmi:id="_st2" base_Class="_ov"/>`)))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	if !strings.Contains(got, "use case def Review;") || strings.Contains(got, "subject overview") {
		t.Errorf("notation names the nested view:\n%s", got)
	}
	want := "the subject is not written: the view Report::Overview is a feature of the part def Report, which only its members can name"
	var found bool
	for _, e := range r.Report.Entries {
		if e.ID != "_uc" {
			continue
		}
		found = true
		if e.Verdict != Approximated || !strings.Contains(e.Note, want) {
			t.Errorf("entry = %s %q, want %s with %q", e.Verdict, e.Note, Approximated, want)
		}
	}
	if !found {
		t.Error("_uc is missing from the report")
	}
}
