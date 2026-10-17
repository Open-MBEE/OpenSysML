package test {
    part def Peg;
    part def Side {
        part p1 : Peg;
        part p2 : Peg;
        part p3 : Peg;
        part p4 : Peg;
        part pegs : Peg [4] = (p1, p2, p3, p4);
    }
    part def Rig {
        part top : Side;
        part front : Side;
        part shared : Peg [2];
        binding [1] bind [0..1] top.pegs = [0..1] shared;
        binding [1] bind [0..1] front.pegs = [0..1] shared;
    }
    part rig : Rig;
}
