// Invalid: a usage redefining an end feature within a connection definition must be an end (KerML 8.3.3.3 validateRedefinitionEndConformance; pilot validateRedefinitionEndConformance).
package S110NonendRedefinesEndInConnectionDef {
    part def A;
    connection def Conn {
        end one : A;
        end two : A;
    }
    connection def Sub :> Conn {
        constant ref q :>> one;
    }
}
