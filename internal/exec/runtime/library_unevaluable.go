package runtime

// registerUnevaluableDeclarations registers, by name and declared signature,
// every library function declaration the runtime cannot evaluate, so a call
// reports the reason.
func registerUnevaluableDeclarations() {
	typeArgument := "the function is abstract and its type parameter is a type reference, not a value (KerML 8.3.4.8 ClassificationExpression): the runtime evaluates the operator notation `x istype T`, `x hastype T`, `x @ T`"
	for _, op := range []string{"istype", "hastype", "@", "@@"} {
		registerUnevaluable("BaseFunctions::"+op, []declaredParam{optionalParam("seq"), param("type")}, typeArgument)
	}
	registerUnevaluable("BaseFunctions::all", nil,
		"the function is abstract and names a type, not a value (KerML 8.3.4.8 ExtentExpression): the runtime evaluates the operator notation `all T`")
	registerUnevaluable("BaseFunctions::as", []declaredParam{optionalParam("seq")},
		"the function is abstract and its type is a type reference, not a value (KerML 8.3.4.8 ClassificationExpression): the runtime evaluates the operator notation `x as T`")
	registerUnevaluable("BaseFunctions::meta", []declaredParam{optionalParam("seq")},
		"the function is abstract and its type is a type reference, not a value (KerML 8.3.4.8 MetadataAccessExpression): the runtime evaluates the operator notation `x meta T`")
	registerUnevaluable("BaseFunctions::[", []declaredParam{optionalParam("x"), optionalParam("y")},
		"'[' is abstract: the runtime evaluates its operator notation `num [unit]`, and its "+
			"QuantityCalculations, VectorCalculations and TensorCalculations specializations take a measurement reference as an argument value")
	registerUnevaluable("ControlFunctions::.", []declaredParam{optionalParam("source")},
		"a feature chain is evaluated from a FeatureChainExpression, not this function")

	noBitwise := "bitwise complement is declared by no function library the runtime applies"
	registerUnevaluable("DataFunctions::~", []declaredParam{param("x")}, noBitwise)
	registerUnevaluable("ScalarFunctions::~", []declaredParam{param("x")}, noBitwise)
}
