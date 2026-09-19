package Mismatch {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Rod;
    part uneven : Rod[2] = (new Rod(), new Rod(), new Rod());

    attribute unevenCount : Natural = size(uneven);
}
