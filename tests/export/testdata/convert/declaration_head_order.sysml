package HeadOrder {
    private import ScalarValues::*;
    metadata def Tag;
    attribute def Mass;
    part def Base {
        attribute m : Mass;
        attribute n : Real;
        attribute p : Real;
    }
    part def Derived :> Base {
        attribute redefines m = 1;
        attribute :>> n : Real = 2;
        attribute q : Real subsets p;
        attribute r : Real = 3;
        attribute :>> p default = 4;
        attribute s : Real [2] :> p;
        attribute t :> p [1..3] : Real;
        #Tag attribute u : Real [1] = 5 {
            doc /* a body member after the head */
        }
    }
}
