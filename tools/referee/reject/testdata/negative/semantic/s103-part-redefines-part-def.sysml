// Invalid: a usage redefines a Feature, so a part redefining a part definition names nothing that position admits (KerML 8.3.3.3 Redefinition::redefinedFeature is typed Feature, a rule SysML usages inherit; pilot fails to link the Redefinition — `Couldn't resolve reference to Feature 'Engine'`).
package S103PartRedefinesPartDef {
    part def Engine;
    part def Car { part e :>> Engine; }
}
