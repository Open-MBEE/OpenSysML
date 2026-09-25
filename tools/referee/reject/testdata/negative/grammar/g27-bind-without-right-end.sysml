// Invalid: BindingConnectorAsUsage requires a connector end after = (SysML.xtext:1020 BindingConnectorAsUsage).
package G27BindWithoutRightEnd {
    part def D {
        attribute a;
        attribute b;
        binding bb bind a = ;
    }
}
