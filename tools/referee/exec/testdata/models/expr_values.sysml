package Probe {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import BaseFunctions::*;
    private import ControlFunctions::*;
    private import NumericalFunctions::*;
    private import ISQ::*;
    private import SI::*;

    attribute n : Integer = 7;
    attribute r : Real = 2.5;
    attribute s : String = "abc";
    attribute b : Boolean = true;
    attribute q : ISQ::MassValue = 3.0 [SI::kg];
    attribute seq : Integer[*] = (3, 1, 2);

    calc def Sum { in x : Integer; in y : Integer; return : Integer = x + y; }
    calc def Fact { in k : Integer; return : Integer = if k <= 1 ? 1 else k * Fact(k - 1); }
    calc def Size { return : Integer = (1, 2, 3)->size(); }
    calc def Sel { return : Integer[*] = (1, 2, 3, 4)->select { in x : Integer; x > 2 }; }
    calc def Single { return : Integer[*] = (1, 2, 3)->select { in x : Integer; x == 2 }; }
    calc def Rev { return : Integer[*] = (1, 2, 3)->including(9); }
    calc def Cat { return : String = "ab" + "cd"; }
    calc def Div { return : Real = 1.0 / 3.0; }
    calc def Neg { return : Integer = -3; }
    calc def Big { return : Integer = 2 ** 40; }
    calc def Cmp { return : Boolean = 1 < 2 and not (3 >= 4); }
    calc def Empty { return : Integer[*] = ()->select { in x : Integer; false }; }
    calc def Range { return : Integer[*] = 1 .. 4; }
    calc def Quant { return : ISQ::MassValue = 3.0 [SI::kg] + 1.0 [SI::kg]; }
}
