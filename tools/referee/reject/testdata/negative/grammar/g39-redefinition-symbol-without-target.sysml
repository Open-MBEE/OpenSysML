// Invalid: Redefinition requires a redefined feature after :>> (KerML.xtext:712 Redefinition).
package G39RedefinitionSymbolWithoutTarget {
    part def D {
        attribute a;
    }
    part def E :> D {
        attribute :>> ;
    }
}
