package ast

import "testing"

func TestExprNodesImplementNode(t *testing.T) {
	var _ Node = &LiteralBool{}
	var _ Node = &LiteralString{}
	var _ Node = &LiteralInteger{}
	var _ Node = &LiteralReal{}
	var _ Node = &LiteralInfinity{}
	var _ Node = &NullExpr{}
	var _ Node = &FeatureReference{}
	var _ Node = &OperatorExpr{}
	var _ Node = &FeatureChainExpr{}
	var _ Node = &IndexExpr{}
	var _ Node = &InvocationExpr{}
	var _ Node = &CollectExpr{}
	var _ Node = &SelectExpr{}
	var _ Node = &ConstructorExpr{}
	var _ Node = &BodyExpr{}
	var _ Node = &SequenceExpr{}
	var _ Node = &MetadataAccessExpr{}
}

func TestOperatorKindString(t *testing.T) {
	if OpAdd.String() != "+" {
		t.Fatalf("OpAdd = %q", OpAdd.String())
	}
	if OpImplies.String() != "implies" {
		t.Fatalf("OpImplies = %q", OpImplies.String())
	}
	if OpConditional.String() != "if" {
		t.Fatalf("OpConditional = %q", OpConditional.String())
	}
}

func TestOperatorExprOperands(t *testing.T) {
	e := &OperatorExpr{Operator: OpAdd, Operands: []Node{&LiteralInteger{Value: "1"}, &LiteralInteger{Value: "2"}}}
	if len(e.Operands) != 2 || e.Operator != OpAdd {
		t.Fatalf("bad operator expr: %+v", e)
	}
}
