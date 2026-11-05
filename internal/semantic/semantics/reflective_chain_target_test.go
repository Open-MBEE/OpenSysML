package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// buildModelWithStdlibUnwalked builds a model over the standard library the way
// the gRPC service and the REPL do: the resolver has walked no document, so
// every name is read on demand.
func buildModelWithStdlibUnwalked(t *testing.T, src string) (*Model, *symbols.Scope) {
	t.Helper()
	idx := stdlibIndex(t)
	name := "t.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	return m, idx.DocumentRoot(name)
}

const chainTargetSrc = `
	package P {
		port def PowerPort { out attribute power : ScalarValues::Real; }
		action def Consume { in power : ScalarValues::Real; in gain : ScalarValues::Real; }
		part def Pt { attribute q : ScalarValues::Real; part inner : In; }
		part def In { attribute z : ScalarValues::Real; calc f { in x : ScalarValues::Real; x } }
		calc def F { in a : ScalarValues::Real; return r : ScalarValues::Real = a; }
		part def Motor {
			port powerPort : PowerPort;
			part p : Pt;
			perform action consume : Consume {
				in power = powerPort.power;
				in gain = 0.5;
			}
			action boost : Consume {
				in power = powerPort.power * 2.0;
			}
			attribute call = F(p.q);
			attribute nested = p.inner.z;
			attribute reached = p.inner.f(1);
			calc viaParameter { in pp : Pt; return r : ScalarValues::Real = pp.q; }
		}
	}
`

// KerML 1.1 Beta 2 §8.3.4.8.4 deriveFeatureChainExpressionTargetFeature: the
// targetFeature of a FeatureChainExpression is the memberElement of its first
// ownedMembership that is not a ParameterMembership. The Membership names the
// chain's member as a member of its operand, read on demand when the document
// walk has not recorded it — for a parameter binding, an operator's operand, an
// invocation's argument, a nested chain and a chain from a parameter alike.
func TestFeatureChainExpressionTargetFeatureIsReadOnDemand(t *testing.T) {
	for _, build := range []struct {
		name  string
		build func(*testing.T, string) (*Model, *symbols.Scope)
	}{
		{"walked", buildModelWithStdlib},
		{"unwalked", buildModelWithStdlibUnwalked},
	} {
		t.Run(build.name, func(t *testing.T) {
			m, root := build.build(t, chainTargetSrc)
			pkg, _ := root.LookupLocal("P")
			motor := member(t, m, pkg, "Motor")
			power := member(t, m, member(t, m, pkg, "PowerPort"), "power")
			q := member(t, m, member(t, m, pkg, "Pt"), "q")
			z := member(t, m, member(t, m, pkg, "In"), "z")
			powerPort := member(t, m, motor, "powerPort")
			chainOf := func(expr *symbols.Symbol) *symbols.Symbol {
				t.Helper()
				if expr == nil {
					t.Fatal("no expression")
				}
				if m.MetaclassOf(expr).Name == "FeatureChainExpression" {
					return expr
				}
				for _, arg := range m.ExpressionArguments(expr) {
					if m.MetaclassOf(arg).Name == "FeatureChainExpression" {
						return arg
					}
				}
				t.Fatalf("%s has no FeatureChainExpression operand", m.MetaclassOf(expr).Name)
				return nil
			}
			for _, c := range []struct {
				name string
				expr *symbols.Symbol
				want *symbols.Symbol
			}{
				{"parameter binding", m.ValueExpression(member(t, m, member(t, m, motor, "consume"), "power")), power},
				{"operator operand", m.ValueExpression(member(t, m, member(t, m, motor, "boost"), "power")), power},
				{"invocation argument", m.ValueExpression(member(t, m, motor, "call")), q},
				{"nested chain", m.ValueExpression(member(t, m, motor, "nested")), z},
				{"chain from a parameter", m.ValueExpression(member(t, m, member(t, m, motor, "viaParameter"), "r")), q},
			} {
				chain := chainOf(c.expr)
				got, ok := m.ReflectiveElements(chain, "targetFeature")
				if !ok || len(got) != 1 || got[0] != c.want {
					t.Errorf("%s: targetFeature = %v (%v), want %s", c.name, got, ok, c.want.Name)
				}
				var membership *symbols.Symbol
				for _, ms := range m.OwnedMemberships(chain) {
					if m.MetaclassOf(ms).Name != "ParameterMembership" {
						membership = ms
						break
					}
				}
				if membership == nil || m.MetaclassOf(membership).Name != "Membership" {
					t.Errorf("%s: the first non-parameter membership is %v", c.name, membership)
					continue
				}
				if el, ok := m.ReflectiveElements(membership, "memberElement"); !ok || len(el) != 1 || el[0] != c.want {
					t.Errorf("%s: Membership.memberElement = %v", c.name, el)
				}
			}
			// The chain's operand names the feature the chain starts from.
			operand := m.ExpressionArguments(chainOf(m.ValueExpression(member(t, m, member(t, m, motor, "consume"), "power"))))
			if got, ok := m.ReflectiveElements(operand[0], "referent"); !ok || len(got) != 1 || got[0] != powerPort {
				t.Errorf("operand referent = %v (%v), want powerPort", got, ok)
			}
			// A call reached through its receiver chains the receiver's features (§8.3.4.8.8).
			reached := m.ValueExpression(member(t, m, motor, "reached"))
			links := m.ChainingFeatures(m.ExpressionChainFeature(reached))
			if len(links) != 3 || links[0] != member(t, m, motor, "p") || links[1] != member(t, m, member(t, m, pkg, "Pt"), "inner") || links[2] != member(t, m, member(t, m, pkg, "In"), "f") {
				t.Errorf("reached call chains %d features", len(links))
			}
		})
	}
}
