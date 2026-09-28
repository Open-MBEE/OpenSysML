// A composite usage nested in a type implicitly subsets the feature of its
// owner its kind nests under (SysML v2 §8.3): vehicle's parts are its
// `subparts`, which are also its `subitems` through the chain that feature
// subsets.
package Vehicle {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel;
    part def Engine { port fuelIn; }
    part def Tank { port fuelOut; }

    part vehicle {
        part wheels : Wheel[4];
        part engine : Engine;
        part tank : Tank;
        interface fuelLine connect engine.fuelIn to tank.fuelOut;
        attribute partCount : Natural = subparts->size();
        attribute itemCount : Natural = subitems->size();
    }
}
