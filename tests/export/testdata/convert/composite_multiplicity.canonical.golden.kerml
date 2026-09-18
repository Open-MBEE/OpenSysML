package CompositeMultiplicity {
    class Wheel;
    class Lugbolt;
    class Vehicle {
        composite feature w : Wheel[4] {
            composite lugbolt : Lugbolt[4..6];
        }
    }
    class Car specializes Vehicle {
        composite frontWheel[2] redefines w {
            composite lugbolt[4] redefines lugbolt;
        }
        composite rearWheel[2] ordered redefines w;
    }
}
