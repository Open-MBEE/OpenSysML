// Invalid: 'abstract' and 'variation' are alternatives of one usage prefix, never both (SysML.xtext:556 RefPrefix).
package G72AbstractVariationUsage {
    part def V;
    abstract variation part v : V;
}
