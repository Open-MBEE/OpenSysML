// Invalid: a usage's name follows its kind keyword; a name written ahead of the keyword begins no body item (SysML.xtext:752 AttributeUsage, :598 UsageDeclaration, :516 DefinitionBodyItem).
package G70NameBeforeUsageKeyword {
    attribute def A;
    part def B {
        foo attribute bar : A;
    }
}
