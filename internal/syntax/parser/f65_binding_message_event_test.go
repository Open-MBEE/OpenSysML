package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F65 covers three forms the pilot accepts and we rejected: a binding connector
// declared before `bind` (SysML.xtext BindingConnectorAsUsage: `UsagePrefix
// ( BindingKeyword UsageDeclaration? )? 'bind' …`), a payload multiplicity on a
// message (Payload: `OwnedFeatureTyping ( OwnedMultiplicity )?`) and a value part
// on an event occurrence reference (EventOccurrenceUsage ends in
// `UsageCompletion` = `ValuePart? UsageBody`).
func TestF65BindingMessageEventParse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			"binding_declaration_with_typing",
			"package P { connection def AB { end a; end b; } part def D { part a; part b; binding ab1 : AB bind a = b; } }",
			[]string{`(Usage kind="binding" name="ab1"`, `(Relationship kind="typing" target=AB`, `(ConnectorEnd target="a"`},
		},
		{
			"binding_typing_without_name",
			"package P { connection def AB { end a; end b; } part def D { part a; part b; binding : AB bind a = b; } }",
			[]string{`(Usage kind="binding" name=""`, `(Relationship kind="typing" target=AB`},
		},
		{
			"binding_body_states_its_ends",
			"package P { class A { feature a : A; feature b : A; binding { end feature references a; end feature references b; } } }",
			[]string{`(Usage kind="binding" name=""`},
		},
		{
			"message_payload_multiplicity",
			"package P { item def Publish; part def D { message m of Publish[1]; } }",
			[]string{`(FlowEnds from="nil" to="nil" payload="Publish" declared=false`, `(Multiplicity range=false`},
		},
		{
			"message_payload_multiplicity_before_type",
			"package P { item def Publish; part def D { message m of [1] Publish; } }",
			[]string{`payload="Publish" declared=false`, `(Multiplicity range=false`},
		},
		{
			"message_payload_declaration_keeps_own_multiplicity",
			"package P { item def Publish; part def D { message m of payload : Publish[1]; } }",
			[]string{`payload="payload" declared=true`, `(Usage kind="attribute" name="payload"`},
		},
		{
			"event_reference_with_value",
			"package P { part def D { part p { event e = m.start; } } }",
			[]string{`(Usage kind="occurrence" name="" `, `(Relationship kind="references" target=e`, `(FeatureChainExpr member="start"`},
		},
		{
			"event_reference_with_body",
			"package P { part def D { part p { event e = m.start { doc /* why */ } } } }",
			[]string{`(Relationship kind="references" target=e`, `(FeatureChainExpr member="start"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, diags := parseFeatureFix(t, tt.name+".sysml", tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			dump := ast.Dump(root)
			for _, want := range tt.want {
				if !strings.Contains(dump, want) {
					t.Fatalf("AST dump does not contain %q:\n%s", want, dump)
				}
			}
		})
	}
}

// A message payload multiplicity belongs to the payload, not to the message, so
// it must not be read as the message's own multiplicity.
func TestF65MessagePayloadMultiplicityOwnership(t *testing.T) {
	root, diags := parseFeatureFix(t, "f65_payload.sysml",
		"package P { item def Publish; part def D { message m of Publish[1]; } }")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
	msg := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if msg.Multiplicity != nil {
		t.Error("message multiplicity is set, want the multiplicity on the payload")
	}
	if msg.FlowEnds == nil || msg.FlowEnds.PayloadMultiplicity == nil {
		t.Fatalf("payload multiplicity = nil, want [1]")
	}
	if msg.FlowEnds.PayloadDecl != nil {
		t.Error("payload declared a feature, want a payload type only")
	}
}

// An event occurrence reference names an existing occurrence and declares no
// name of its own, so its value part must not turn it into a declaration.
func TestF65EventOccurrenceReferenceValue(t *testing.T) {
	root, diags := parseFeatureFix(t, "f65_event.sysml",
		"package P { part def D { part p { event e = m.start; } } }")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	part := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	ev := part.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if ev.Kind != ast.UsageOccurrence || !ev.IsEvent {
		t.Errorf("got kind=%v event=%t, want an event occurrence usage", ev.Kind, ev.IsEvent)
	}
	if ev.Ident.Name != "" {
		t.Errorf("name = %q, want the reference to name it", ev.Ident.Name)
	}
	if len(ev.Relationships) != 1 || ev.Relationships[0].Kind != ast.RelReferences {
		t.Fatalf("relationships = %v, want one references", ev.Relationships)
	}
	if ev.Value == nil {
		t.Error("value = nil, want m.start")
	}
}

// Malformed F65 forms must produce diagnostics, never a panic.
func TestF65Negative(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"binding_typing_no_target", "package P { part def D { binding ab1 : ; } }"},
		{"binding_end_missing", "package P { part def D { binding : AB bind = b; } }"},
		{"binding_typing_no_bind", "package P { part def D { binding ab1 : AB } }"},
		{"binding_declaration_no_value", "package P { part def D { binding ab1 : AB bind a = ; } }"},
		{"binding_at_eof", "package P { part def D { binding ab1 : AB bind"},
		{"message_payload_unclosed_multiplicity", "package P { part def D { message m of Publish[1 ; } }"},
		{"message_payload_multiplicity_no_type", "package P { part def D { message m of [1] ; } }"},
		{"message_payload_at_eof", "package P { part def D { message m of Publish["},
		{"event_value_missing", "package P { part def D { part p { event e = ; } } }"},
		{"event_reference_missing", "package P { part def D { part p { event .start; } } }"},
		{"event_reference_multiplicity_unclosed", "package P { part def D { part p { event e[ = m.start; } } }"},
		{"event_unterminated", "package P { part def D { part p { event e = m.start } } }"},
		{"event_at_eof", "package P { part def D { part p { event e ="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			sf := source.New(tt.name+".sysml", []byte(tt.src))
			p := New(sf)
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected diagnostics for %q, got none", tt.src)
			}
		})
	}
}

// Truncating each accepted form at every token must still return a tree.
func TestF65Robustness(t *testing.T) {
	sources := []string{
		"package P { part def D { binding ab1 : AB bind a = b; } }",
		"package P { part def D { message m of Publish[1]; } }",
		"package P { part def D { part p { event e = m.start; } } }",
		"package P { class A { binding { end feature references a; } } }",
	}
	for _, src := range sources {
		for i := 1; i <= len(src); i++ {
			prefix := src[:i]
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic on %q: %v", prefix, r)
					}
				}()
				p := New(source.New("f65_robust.sysml", []byte(prefix)))
				if p.ParseFile() == nil {
					t.Fatalf("ParseFile returned nil for %q", prefix)
				}
			}()
		}
	}
}
