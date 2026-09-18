// KerML.xtext Invariant: FeaturePrefix 'inv' ( 'true' | isNegated ?= 'false' )?
// ExpressionDeclaration FunctionBody. The truth keyword after `inv` is the
// invariant's polarity, not its name: `false` sets isNegated, `true` is the default.
package InvariantPolarity {
	class Account {
		feature balance : ScalarValues::Real;
		inv true positive { balance >= 0 }
		inv false overdrawn { balance < 0 }
		inv bounded { balance <= 1000 }
		abstract inv false <'ao'> abstractOverdrawn { balance < 0 }
	}
}
