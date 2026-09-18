package Values {
    calc def Total {
        in x : Real;
        in y : Real;
        return result : Real;
    }
    part def Sized {
        attribute size : Real = 4.5;
        attribute label : String = "front";
        attribute total : Real = 1.0 + 2.0;
    }
}
