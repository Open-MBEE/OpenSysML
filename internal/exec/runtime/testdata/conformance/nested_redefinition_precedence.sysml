package test {
    private import ScalarValues::Real;
    part def Leaf { attribute value : Real default = 1.0; }
    part def Mid {
        part leaf : Leaf;
        attribute scale : Real default = 1.0;
        attribute :>> leaf.value = 5.0;
    }
    part def Base { part mid : Mid; attribute :>> mid.scale = 4.0; }
    part def Derived :> Base { attribute :>> mid.scale = 9.0; }
    // The subtype's chain wins over the base's, and a chain carried down from
    // the usage wins over one the intermediate type's own body declares.
    part top : Derived { attribute :>> mid.leaf.value = 99.0; }
}
