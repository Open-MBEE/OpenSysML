// Invalid: a usage redefining an end feature within a connection usage must be an end (KerML 8.3.3.3 validateRedefinitionEndConformance; pilot validateRedefinitionEndConformance).
package S111NonendRedefinesEndInConnectionUsage {
    part def A;
    connection def Conn {
        end one : A;
        end two : A;
    }
    part ctx {
        part x : A;
        part y : A;
        connection c : Conn connect x to y {
            constant ref q :>> one;
        }
    }
}
