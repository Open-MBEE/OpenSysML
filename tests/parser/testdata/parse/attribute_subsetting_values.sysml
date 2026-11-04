package P {
    private import ScalarValues::*;

    part def Values {
        attribute q = 4;
        attribute r [1] :> q;
        attribute faces [6] = (1..6);
        attribute top [1] :> faces;
        attribute codes : Integer [3] = (7, 8, 9);
        attribute late : Integer [2] = (8, 9);
        attribute picked : Integer [1] :> codes;
        binding [1] bind [0..1] picked = [0..1] late;
    }
}
