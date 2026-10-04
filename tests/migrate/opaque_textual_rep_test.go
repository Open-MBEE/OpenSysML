package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func opaqueActionDocument(t *testing.T, body string) *migrate.Result {
	t.Helper()
	return migrateDocument(t, `
    <packagedElement xmi:type="uml:Activity" xmi:id="_activity" name="Run">
      <node xmi:type="uml:InitialNode" xmi:id="_initial"/>
      <node xmi:type="uml:OpaqueAction" xmi:id="_opaque" name="write">`+body+`</node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_first" source="_initial" target="_opaque"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_last" source="_opaque" target="_final"/>
    </packagedElement>`, "")
}

func TestOpaqueActionTextualRepresentations(t *testing.T) {
	t.Run("translated", func(t *testing.T) {
		r := opaqueActionDocument(t, `<language>JavaScript</language><body>x = 1;</body>`)
		wantLine(t, r.Notation, `rep language "JavaScript" /* x = 1; */`)
		wantLine(t, r.Notation, "assign x := 1;")
		wantClean(t, "t.sysml", r)
	})

	t.Run("untranslated", func(t *testing.T) {
		r := opaqueActionDocument(t, `<language>Java</language><body>work();</body>`)
		wantLine(t, r.Notation, `rep language "Java" /* work(); */`)
		wantNoLine(t, r.Notation, "body not migrated")
		wantNote(t, r, "_opaque", migrate.Approximated, "the body is kept as a textual representation, which is not executed")
		wantClean(t, "t.sysml", r)
	})

	t.Run("without language", func(t *testing.T) {
		r := opaqueActionDocument(t, `<body>not a statement</body>`)
		wantLine(t, r.Notation, "body not migrated")
		wantLine(t, r.Notation, "not a statement")
		wantNote(t, r, "_opaque", migrate.Approximated, "the body is kept as a comment")
		wantClean(t, "t.sysml", r)
	})

	t.Run("pairs by index", func(t *testing.T) {
		r := opaqueActionDocument(t, `<language>Java</language><body>first();</body><language>Python</language><body>second()</body>`)
		wantLine(t, r.Notation, `rep language "Java" /* first(); */`)
		wantLine(t, r.Notation, `rep language "Python" /* second() */`)
		wantNoLine(t, r.Notation, "body not migrated")
		wantClean(t, "t.sysml", r)
	})

	t.Run("attribute fallback", func(t *testing.T) {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Activity" xmi:id="_activity" name="Run">
      <node xmi:type="uml:InitialNode" xmi:id="_initial"/>
      <node xmi:type="uml:OpaqueAction" xmi:id="_opaque" name="write" language="Java" body="work();"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_first" source="_initial" target="_opaque"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_last" source="_opaque" target="_final"/>
    </packagedElement>`, "")
		wantLine(t, r.Notation, `rep language "Java" /* work(); */`)
		wantClean(t, "t.sysml", r)
	})

	t.Run("empty", func(t *testing.T) {
		r := opaqueActionDocument(t, "")
		wantNoLine(t, r.Notation, "body not migrated")
		wantNoLine(t, r.Notation, "rep language")
		wantNote(t, r, "_opaque", migrate.Mapped, "its body is empty, so it is written as an action that does nothing")
		wantClean(t, "t.sysml", r)
	})

	t.Run("comment terminator", func(t *testing.T) {
		r := opaqueActionDocument(t, `<language>Python</language><body>print("*/")</body>`)
		wantLine(t, r.Notation, `rep language "Python" /* print("* /") */`)
		wantClean(t, "t.sysml", r)
	})
}

func TestOpaqueBehaviorTextualRepresentations(t *testing.T) {
	t.Run("action definition", func(t *testing.T) {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:OpaqueBehavior" xmi:id="_behavior" name="Work">
      <language>Java</language>
      <body>work();</body>
    </packagedElement>`, "")
		wantLine(t, r.Notation, "action def Work {")
		wantLine(t, r.Notation, `rep language "Java" /* work(); */`)
		wantNoLine(t, r.Notation, "body not migrated")
		wantNote(t, r, "_behavior", migrate.Approximated, "the body is kept as a textual representation, which is not executed")
		wantClean(t, "t.sysml", r)
	})

	t.Run("calculation definition", func(t *testing.T) {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:OpaqueBehavior" xmi:id="_calculation" name="Sum">
      <language>JavaScript</language>
      <body>1 + 2</body>
    </packagedElement>`, "")
		wantLine(t, r.Notation, `rep language "JavaScript" /* 1 + 2 */`)
		rep := strings.Index(string(r.Notation), `rep language "JavaScript"`)
		result := strings.LastIndex(string(r.Notation), "1 + 2")
		if rep < 0 || result < 0 || rep >= result {
			t.Errorf("textual representation must precede the calc result:\n%s", r.Notation)
		}
		wantClean(t, "t.sysml", r)
	})

	t.Run("empty behaviors", func(t *testing.T) {
		for _, kind := range []string{"OpaqueBehavior", "FunctionBehavior"} {
			t.Run(kind, func(t *testing.T) {
				r := migrateDocument(t, `
    <packagedElement xmi:type="uml:`+kind+`" xmi:id="_behavior" name="Empty"/>`, "")
				wantNoLine(t, r.Notation, "body not migrated")
				wantNoLine(t, r.Notation, "rep language")
				wantNote(t, r, "_behavior", migrate.Mapped, "")
				wantClean(t, "t.sysml", r)
			})
		}
	})

	t.Run("operation-written definition", func(t *testing.T) {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_owner" name="Owner">
      <ownedOperation xmi:type="uml:Operation" xmi:id="_operation" name="Execute" method="_method"/>
      <ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="_method" name="Method">
        <language>Java</language>
        <body>work();</body>
      </ownedBehavior>
    </packagedElement>`, `<sysml:Block xmi:id="_block" base_Class="_owner"/>`)
		wantLine(t, r.Notation, `rep language "Java" /* work(); */`)
		wantClean(t, "t.sysml", r)
	})
}
