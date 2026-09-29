// Invalid: RefPrefix modifiers follow their grammar order (SysML.xtext:556-566 RefPrefix, :563 BasicUsagePrefix).
package G81ActionParameterModifierOrder {
    attribute def T;
    action def A { in constant derived p : T; }
}
