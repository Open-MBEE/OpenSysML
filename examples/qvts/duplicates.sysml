package Duplicates {
    private import ScalarValues::*;

    part def Instrument {
        attribute mass : Real[1];
    }

    part repeated : Instrument;
    part repeated : Instrument;

    part doubleMass : Instrument {
        attribute :>> mass = 3.0;
        attribute :>> mass = 4.0;
    }
}
