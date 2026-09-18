package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// TestUnsupportedConversionMessages pins the text of a conversion refusal: the
// position of the construct (or its IRI when the graph is what is refused), its
// SysML name rather than a Go type, and the reason it has no place in the graph.
func TestUnsupportedConversionMessages(t *testing.T) {
	cases := []struct {
		name string
		src  string
		// edit, when set, corrupts the graph src converts to; the refusal is
		// then the decoder's, reading that graph back.
		edit func(t *testing.T, turtle []byte) []byte
		want []string
	}{{
		// A `snapshot` whose graph types no portion would come back as `occurrence`.
		name: "untyped_snapshot",
		src:  "package P {\n\tpart def A {\n\t\tsnapshot s;\n\t}\n}",
		edit: func(t *testing.T, turtle []byte) []byte {
			return editTurtle(t, turtle, "    sysml:portionKind \"snapshot\" ;\n", "")
		},
		want: []string{"cannot convert the `snapshot` declaration <urn:sysmlv2:element:P__A__s>",
			"it has no sysml:portionKind, not the \"snapshot\" its keyword states",
			"cannot be rebuilt without declaring something else"},
	}, {
		// An `event m.start` whose reference is gone has nothing to write after `event`.
		name: "event_without_reference",
		src:  "package P {\n\tpart def A {\n\t\toccurrence m;\n\t\tevent m;\n\t}\n}",
		edit: func(t *testing.T, turtle []byte) []byte {
			return editTurtle(t, turtle, "    sysml:references elmt:P__A__m ;\n", "")
		},
		want: []string{"cannot convert the `event` declaration <urn:sysmlv2:element:P__A___401>",
			"it neither declares a name nor names the feature it refers to (sysml:references)",
			"cannot be rebuilt from the graph"},
	}, {
		// A `perform a` whose reference is gone would come back as a feature named `perform`.
		name: "perform_without_reference",
		src:  "package P {\n\tpart def A {\n\t\taction a;\n\t\tperform a;\n\t}\n}",
		edit: func(t *testing.T, turtle []byte) []byte {
			return editTurtle(t, turtle, "    sysml:references elmt:P__A__a ;\n", "")
		},
		want: []string{"cannot convert the `perform` declaration <urn:sysmlv2:element:P__A___401>",
			"it neither declares a name nor names the feature it refers to (sysml:references)"},
	}, {
		// An `assert c` whose reference is gone has nothing to write after `assert`.
		name: "assert_without_reference",
		src:  "package P {\n\tpart def A {\n\t\tconstraint c;\n\t\tassert c;\n\t}\n}",
		edit: func(t *testing.T, turtle []byte) []byte {
			return editTurtle(t, turtle, "    sysml:references elmt:P__A__c ;\n", "")
		},
		want: []string{"cannot convert the `assert` declaration <urn:sysmlv2:element:P__A___401>",
			"it neither declares a name nor names the feature it refers to (sysml:references)"},
	}, {
		name: "duplicate_declaration",
		src:  "package P {\n\tpart def A {\n\t\tattribute x : Real;\n\t\tattribute x : Real;\n\t}\n}",
		want: []string{"cannot convert the duplicate declaration of \"x\" at m.sysml:4:3",
			"two members of one namespace cannot share it"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := export.Convert("m.sysml", []byte(tc.src), export.FormatSysML, export.FormatTurtle)
			if tc.edit != nil {
				if err != nil {
					t.Fatalf("to turtle: %v", err)
				}
				_, err = export.Convert("m.ttl", tc.edit(t, out), export.FormatTurtle, export.FormatSysML)
			}
			if err == nil {
				t.Fatal("expected the conversion to be refused")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err.Error())
				}
			}
			if strings.Contains(err.Error(), "*ast.") {
				t.Errorf("error names a Go type: %s", err.Error())
			}
		})
	}
}

// TestFormatRemedyNamesBothSurfaces pins that a format that cannot be told from
// the name advises the flag of the surface asked and the extension remedy the
// other surface uses, so the two agree.
func TestFormatRemedyNamesBothSurfaces(t *testing.T) {
	_, err := export.FormatOfPath("model.txt")
	if err == nil {
		t.Fatal("expected an unknown-format error")
	}
	advised := export.Advise(err, "pass -from, or "+export.ExtensionAdvice)
	want := `cannot tell the format of "model.txt": expected .sysml, .kerml or .ttl, ` +
		"so pass -from, or name the file with a .sysml, .kerml or .ttl extension"
	if advised.Error() != want {
		t.Errorf("err = %q; want %q", advised.Error(), want)
	}
}
