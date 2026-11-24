package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const annotationValueFixture = `package P {
	metadata def M { attribute label : String = "default"; }
	part thing;
	@M about thing { label = "old"; }
}`

// A set-annotation-value operation rewrites a usage binding, not the
// definition's default, and reports the exact bytes it changed.
func TestSetAnnotationValueRewritesTheUsageBinding(t *testing.T) {
	m := loadContent(t, "annotation.sysml", annotationValueFixture)
	result, err := Apply(m, []Operation{
		SetAnnotationValue("P::thing", "P::M", "label", `"new"`),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := strings.Replace(annotationValueFixture, `label = "old"`, `label = "new"`, 1)
	if got := string(result.Content); got != want {
		t.Fatalf("edited model =\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(string(result.Content), `attribute label : String = "default"`) {
		t.Fatal("the metadata definition's default was rewritten")
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applied = %+v, want one replacement", result.Applied)
	}
	applied := result.Applied[0]
	if applied.OldText != `"old"` || applied.NewText != `"new"` {
		t.Fatalf("applied = %+v, want the bound value \"old\" replaced by \"new\"", applied)
	}
	if applied.Target != "P::M::label" {
		t.Fatalf("applied target = %q, want P::M::label", applied.Target)
	}
	expectedOffset := strings.Index(annotationValueFixture, `"old"`)
	if applied.Span.Offset != expectedOffset || applied.Span.Len != len(`"old"`) {
		t.Fatalf("applied span = %+v, want offset %d len %d", applied.Span, expectedOffset, len(`"old"`))
	}
	sf := source.New("edited.sysml", result.Content)
	p := parser.New(sf)
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("edited model does not parse clean: %v", p.Diagnostics)
	}
}

func TestSetAnnotationValueAddsValueToUnboundFeature(t *testing.T) {
	m := loadContent(t, "annotation.sysml", `package P {
	metadata def Tag { attribute label : String = "default"; }
	part p { @Tag { label; } }
}`)
	result, err := Apply(m, []Operation{
		SetAnnotationValue("P::p", "P::Tag", "label", `"new"`),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	text := string(result.Content)
	if !strings.Contains(text, `label = "new";`) {
		t.Fatalf("edited model =\n%s\nwant label = \"new\";", text)
	}
	if !strings.Contains(text, `attribute label : String = "default"`) {
		t.Fatal("the metadata definition's default was rewritten")
	}
	sf := source.New("edited.sysml", result.Content)
	p := parser.New(sf)
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("edited model does not parse clean: %v", p.Diagnostics)
	}
}

func TestSetAnnotationValueRewritesAnInlineBinding(t *testing.T) {
	m := loadContent(t, "annotation.sysml", `package Q {
	metadata def Tag { attribute note : String; }
	part device { @Tag { note = "before"; } }
}`)
	result, err := Apply(m, []Operation{
		SetAnnotationValue("Q::device", "Q::Tag", "note", `"after"`),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(result.Content), `note = "after"`) ||
		strings.Contains(string(result.Content), `note = "before"`) {
		t.Fatalf("edited model =\n%s\nwant the inline binding rewritten", result.Content)
	}
}

// Naming a feature no annotation of that type binds is a typed refusal, not an
// edit of the metadata definition's like-named default.
func TestSetAnnotationValueRefusesAnUnboundFeature(t *testing.T) {
	m := loadContent(t, "annotation.sysml", annotationValueFixture)
	_, err := Apply(m, []Operation{
		SetAnnotationValue("P::thing", "P::M", "missing", `"new"`),
	})
	if e := editError(t, err); e.Failure != FailureUnknownTarget {
		t.Fatalf("error = %v, want failure %v", e, FailureUnknownTarget)
	}
}
