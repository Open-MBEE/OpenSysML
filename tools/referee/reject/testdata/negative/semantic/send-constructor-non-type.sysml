// Invalid: a constructor must instantiate a type, not a package (KerML 8.3.4.8 InstantiationExpression validateInstantiationExpressionInstantiatedType; pilot validateInstantiationExpressionInstantiatedType).
package SendConstructorNonType {
    package Signals {
        item def Sig;
    }
    part def Receiver;
    action def A {
        part target : Receiver;
        action s send new Signals() to target;
    }
}
