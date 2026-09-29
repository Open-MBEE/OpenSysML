// Invalid: RefPrefix modifiers precede the optional 'ref' (SysML.xtext:556-566 RefPrefix, :563 BasicUsagePrefix).
package G80ActionParameterRefPrefixOrder {
    attribute def T;
    action def A { in ref derived p : T; }
}
