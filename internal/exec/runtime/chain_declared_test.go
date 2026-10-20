package runtime

import (
	"errors"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"testing"
)

// packageUsageChainModel binds values on the members of package-level usages no
// object stands for, beside members nothing binds, so a chain can read either.
const packageUsageChainModel = `
package test {
	private import ScalarValues::*;

	attribute def Details { attribute limit : Real; attribute other : Real; }
	requirement def Limit {
		attribute maximum : Real;
		attribute margin : Real = 5.0;
		attribute unset : Real;
		attribute details : Details { :>> limit default = 4.0; }
		attribute deep : Details;
		require constraint { maximum > 0.0 }
	}
	requirement costReq : Limit { :>> maximum = 100.0; }
	requirement optionalReq : Limit [0..1] { :>> maximum = 7.0; }
	requirement bareReq : Limit;
	requirement inherit : Limit { attribute kept :>> margin; attribute :>> deep { :>> other = 6.0; } }
	requirement chained : Limit { attribute :>> details.limit = 9.0; }
	requirement def Tight :> Limit { attribute :>> details.limit = 7.0; }
	requirement tight : Tight;
	requirement bodyWins : Limit { attribute :>> details { :>> limit = 3.0; } attribute :>> details.limit = 5.0; }
	attribute def Details2 :> Details { attribute nick :>> limit; }
	attribute template : Details { :>> limit = 1.0; }
	requirement def Bound { attribute details : Details default = template; attribute d2 : Details2; }
	requirement boundReq : Bound { attribute :>> details.limit = 9.0; attribute :>> d2.nick = 9.0; }
	requirement boundPlain : Bound;

	attribute def Dims { attribute w : Real; attribute h : Real = 2.0; }
	part def Product { attribute mass : Real = 3.0; attribute dims : Dims { :>> w = 4.0; } }
	part product : Product;
	port def Plug { attribute pins : Real = 9.0; }
	port plug : Plug;
	item crate : Product;

	requirement selfRef : Limit { :>> maximum = test::selfRef.maximum; }

	requirement cheap { require constraint { 50.0 <= costReq.maximum } }
	requirement dear { require constraint { 150.0 <= costReq.maximum } }
	requirement unknown { require constraint { 50.0 <= bareReq.maximum } }
}
`

// A chain through a package-level usage reads the value its member binds, as the
// qualified name does, for a requirement as for a part, port or item usage.
func TestChainThroughPackageUsageReadsBoundMember(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, packageUsageChainModel)
	ctx := NewContext(typedModel(model, resolver), 10000)
	pkg, _ := root.LookupLocal("test")

	for src, want := range map[string]string{
		"costReq.maximum":                           "100.0",
		"test::costReq.maximum":                     "100.0",
		"test::costReq::maximum":                    "100.0",
		"costReq.margin":                            "5.0",
		"inherit.kept":                              "5.0",
		"costReq.details.limit":                     "4.0",
		"inherit.deep.other":                        "6.0",
		"chained.details.limit":                     "9.0",
		"tight.details.limit":                       "7.0",
		"bodyWins.details.limit":                    "3.0",
		"boundReq.details.limit":                    "9.0",
		"boundPlain.details.limit":                  "1.0",
		"boundReq.d2.nick":                          "9.0",
		"product.mass":                              "3.0",
		"product.dims.w":                            "4.0",
		"product.dims.h":                            "2.0",
		"plug.pins":                                 "9.0",
		"crate.mass":                                "3.0",
		"costReq.maximum - 20.0":                    "80.0",
		"80.0 <= costReq.maximum":                   "true",
		"120.0 <= costReq.maximum":                  "false",
		"costReq.maximum == test::costReq::maximum": "true",
	} {
		val, err := ctx.EvalWithScope(parseExpr(t, src), pkg.Scope)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := FormatValue(val); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	// The object a requirement chain was read from is taken back with its probe;
	// the parts read as occurrences stay, as before.
	for _, inst := range ctx.instances {
		if usage, ok := inst.Type.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageRequirement {
			t.Errorf("requirement %s materialized by a model-level chain read", inst.Type.Name)
		}
	}
}

// A member nothing binds stays undetermined through the chain, as does a member
// of a usage whose instances are not exactly one: no value is invented. An optional
// member nothing values holds none: the empty sequence, closed over the model.
func TestChainThroughPackageUsageWithoutBoundMemberIsUndetermined(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, packageUsageChainModel)
	ctx := NewContext(typedModel(model, resolver), 10000)
	pkg, _ := root.LookupLocal("test")

	for src, count := range map[string]string{
		"costReq.unset":                    "[1]",
		"bareReq.maximum":                  "[1]",
		"product.dims.w + bareReq.maximum": "[1]",
		"costReq.details.other":            "[1]",
		"inherit.deep.limit":               "[1]",
		"tight.details.other":              "[1]",
		"boundReq.details.other":           "[1]",
		"boundPlain.d2.nick":               "[1]",
	} {
		val, err := ctx.EvalWithScope(parseExpr(t, src), pkg.Scope)
		wantUndetermined(t, src, val, err, count)
	}
	val, err := ctx.EvalWithScope(parseExpr(t, "optionalReq.maximum"), pkg.Scope)
	wantFormatted(t, "optionalReq.maximum", val, err, "[]")

	_, err = ctx.EvalWithScope(parseExpr(t, "costReq.nonexistent"), pkg.Scope)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("costReq.nonexistent: err = %v; want ErrUnresolvedReference", err)
	}
	_, err = ctx.EvalWithScope(parseExpr(t, "selfRef.maximum"), pkg.Scope)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("selfRef.maximum: err = %v; want ErrCyclicFeatureValue", err)
	}
}

// A require constraint reading a criterion through a package-level requirement
// usage is satisfied or violated by the bound value, and left unevaluated by none.
func TestRequireConstraintReadsPackageUsageMember(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, packageUsageChainModel))
	for fqn, want := range map[string]error{
		"test::cheap":   nil,
		"test::dear":    ErrViolated,
		"test::unknown": ErrNoValue,
	} {
		if _, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil); !errors.Is(err, want) {
			t.Errorf("%s: error = %v, want %v", fqn, err, want)
		}
	}
}
