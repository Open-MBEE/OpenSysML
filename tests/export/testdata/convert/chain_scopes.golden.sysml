package Gen {
    part def G1 {
        attribute x;
        attribute y;
    }
    part def G2 {
        attribute x;
    }
}
package Chains {
    part def Wheel :> Gen::G1, Gen::G2;
    part def Axle {
        part hub : Wheel;
    }
    part def Car {
        part w : Wheel;
        part axle : Axle;
        attribute a = w.Gen::G1::x;
        attribute b = w.Gen::G2::x;
        attribute c = w.y;
        attribute d = axle.hub.Gen::G2::x;
        attribute e = axle.hub.y;
        part spare :> w;
        attribute f = spare.y;
    }
}
