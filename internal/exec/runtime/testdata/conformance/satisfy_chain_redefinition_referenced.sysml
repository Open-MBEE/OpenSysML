// A chain redefinition a referenced requirement usage declares on the satisfying
// feature (`requirement full : Full { attribute redefines vehicle1.fuel = 1300; }`)
// holds when `satisfy full by vehicle1;` checks it, as one the satisfy usage
// declares itself does; a second satisfaction of the same part by a requirement
// without the chain reads the part's own value.
package ReferencedProbe {
    private import ScalarValues::Integer;
    part def Vehicle {
        attribute fuel : Integer default 0;
    }
    part vehicle1 : Vehicle;
    requirement def Full {
        subject vehicle : Vehicle;
        attribute target : Integer default 1300;
        require constraint { vehicle.fuel == target }
    }
    requirement full : Full {
        attribute redefines vehicle1.fuel = 1300;
    }
    requirement plain : Full;
    part context {
        satisfy full by vehicle1;
        satisfy plain by vehicle1;
    }
}
