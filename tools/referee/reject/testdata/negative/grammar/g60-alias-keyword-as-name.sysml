// Invalid: an alias name comes from the ID terminal, so 'part' cannot spell it
// (SysML.xtext AliasMember / Identification, terminal ID).
package G60AliasKeywordAsName {
    part def B;
    alias part for G60AliasKeywordAsName::B;
}
