// Invalid: A positional argument must follow the comma (KerMLExpressions.xtext:456 PositionalArgumentList).
package G54ArgumentListTrailingComma {
    calc def F { in a; in b; return : Integer; }
    attribute y = F(1,);
}
