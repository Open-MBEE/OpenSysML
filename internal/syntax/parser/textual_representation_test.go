package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A textual representation is a body member wherever a member is expected, in
// both file kinds, named (`rep <name> language "…"`) or anonymous
// (`language "…"`, KerML :103).
func TestTextualRepresentationForms(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
		want string
	}{
		{
			"named_namespace_member_kerml",
			"named.kerml",
			`package P { rep asJava language "java" /* class W {} */ }`,
			`TextualRepresentation language="\"java\"" name="asJava"`,
		},
		{
			"anonymous_namespace_member_kerml",
			"anonymous.kerml",
			`package P { language "alf" /* WriteLine("x"); */ }`,
			`TextualRepresentation language="\"alf\""`,
		},
		{
			"anonymous_behavior_member_kerml",
			"behavior.kerml",
			`package P { behavior setX { in newX; language "alf" /* x = newX; */ } }`,
			`TextualRepresentation language="\"alf\""`,
		},
		{
			"named_definition_member_sysml",
			"named.sysml",
			`package P { part def C { rep inOCL language "ocl" /* self.x > 0.0 */ } }`,
			`TextualRepresentation language="\"ocl\"" name="inOCL"`,
		},
		{
			"anonymous_definition_member_sysml",
			"anonymous.sysml",
			`package P { part def C { language "alf" /* x = 1; */ } }`,
			`TextualRepresentation language="\"alf\""`,
		},
		{
			"short_name_kerml",
			"shortname.kerml",
			`package P { rep <ocl> inOCL language "ocl" /* true */ }`,
			`TextualRepresentation language="\"ocl\"" name="<ocl> inOCL"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.file, []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if dump := ast.Dump(root); !strings.Contains(dump, tt.want) {
				t.Fatalf("AST dump does not contain %q:\n%s", tt.want, dump)
			}
		})
	}
}

// A representation missing its language string or its comment body is
// diagnosed and recovered from, in either file kind.
func TestNegativeTextualRepresentation(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
	}{
		{"named_without_language_string", "a.kerml", `package P { class C { rep bad language /* body */ } }`},
		{"named_without_body", "b.kerml", `package P { class C { rep bad language "ocl" } }`},
		{"anonymous_without_body", "c.kerml", `package P { language "ocl" }`},
		{"anonymous_without_body_sysml", "d.sysml", `package P { part def C { language "ocl" } }`},
		{"unterminated_body", "e.sysml", `package P { rep r language "ocl" /* unterminated`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New(tt.file, []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
		})
	}
}
