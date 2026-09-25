package resolve_test

import "testing"

// The corpus pattern: the body of an association end that redefines an
// inherited end names a member of a member of the redefined end
// (F72: `member feature Product_Account1 subsets Product_Account`).
const f72General = `package P {
	class ShoppingCart;
	class Product;
	class Account;

	assoc A {
		end cart: ShoppingCart[1] {
			member feature inCart: ShoppingCart[0..1] featured by Product_Account {
				member feature Product_Account : Account featured by Product {
					member feature deepThing : Account featured by Product;
				}
			}
		}
		end feature account : Account[1];
	}
`

func f72Doc(t *testing.T, name, body string) (rejects bool) {
	t.Helper()
	r, _, _ := resolvedDocNamed(t, name+".kerml", f72General+body+"}")
	for _, d := range r.Diagnostics {
		t.Logf("diag: %s", d.Message)
	}
	return len(r.Diagnostics) != 0
}

// A redefining end's body sees what is nested in the end it redefines, at any
// depth: the reference here names a member of a member of the redefined end.
func TestF72NestedMemberOfRedefinedEnd(t *testing.T) {
	if f72Doc(t, "explicit", `	assoc B specializes A {
		end cart: ShoppingCart[1] redefines cart {
			member feature inCart1: ShoppingCart[0..1] featured by Product_Account1 {
				member feature Product_Account1 subsets Product_Account : Account featured by Product;
			}
		}
		end feature account : Account[1];
	}
`) {
		t.Error("unexpected diagnostics above")
	}
	if f72Doc(t, "deeper", `	assoc B specializes A {
		end cart: ShoppingCart[1] redefines cart {
			member feature inCart1: ShoppingCart[0..1] {
				member feature Product_Account1 subsets Product_Account : Account featured by Product {
					member feature deepThing1 subsets deepThing : Account featured by Product;
				}
			}
		}
		end feature account : Account[1];
	}
`) {
		t.Error("unexpected diagnostics above")
	}
}

// An association end redefines the corresponding end of the association it
// specializes without saying so (KerML 8.4.4.6), so the same names are visible
// in a body that never spells the redefinition out.
func TestF72NestedMemberThroughImplicitEndRedefinition(t *testing.T) {
	if f72Doc(t, "implicit", `	assoc B specializes A {
		end cart: ShoppingCart[1] {
			member feature inCart1: ShoppingCart[0..1] {
				member feature Product_Account1 subsets Product_Account : Account featured by Product;
			}
		}
		end feature account : Account[1];
	}
`) {
		t.Error("unexpected diagnostics above")
	}
}

// The exposure is confined to redefinition: the specializing association's own
// body does not see features nested in what it inherits, and neither does a
// plain specialization of the association — as the reference confirms.
func TestF72NestedMemberNotVisibleWithoutRedefinition(t *testing.T) {
	if !f72Doc(t, "own-body", `	assoc B specializes A {
		end feature cart2: ShoppingCart[1];
		end feature account2 : Account[1];
		member feature y subsets Product_Account;
	}
`) {
		t.Error("expected an unresolved-reference diagnostic in the specializing association's own body")
	}
	if !f72Doc(t, "plain-specialization", `	class D specializes A {
		feature z subsets Product_Account;
	}
`) {
		t.Error("expected an unresolved-reference diagnostic in a plain specialization")
	}
}

// A name nested nowhere under the redefined end stays unresolved.
func TestF72UnknownNameStillFails(t *testing.T) {
	if !f72Doc(t, "unknown", `	assoc B specializes A {
		end cart: ShoppingCart[1] redefines cart {
			member feature inCart1: ShoppingCart[0..1] {
				member feature Product_Account1 subsets NoSuchThing : Account featured by Product;
			}
		}
		end feature account : Account[1];
	}
`) {
		t.Error("expected an unresolved-reference diagnostic for a name nested nowhere")
	}
}

// A redefinition whose own target is unresolvable, and a self-redefinition,
// must degrade to diagnostics rather than recurse or panic.
func TestF72UnresolvableRedefinitionNoPanic(t *testing.T) {
	r, _, _ := resolvedDocNamed(t, "cycle.kerml", `package P {
		class C;
		assoc A {
			end cart: C[1] redefines nowhere {
				member feature f subsets alsoNowhere;
			}
			end feature other : C[1] redefines other;
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for unresolvable redefinition targets")
	}
}
