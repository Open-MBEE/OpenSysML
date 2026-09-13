// A calc over `x meta T` (KerML 7.4.9.2): the trace renders the result's
// annotation object before the reflective metaobject (KerML 8.3.4.8.15),
// which prints as the element's qualified name and its metaclass, and the
// metaobject's features read through the ordinary member path.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    metadata def Safety {
        attribute level : Integer = 2;
    }

    part def Vehicle {
        attribute mass : Real;
    }

    part seatBelt : Vehicle {
        @Safety { level = 4; }
    }

    calc def Describe {
        attribute everything [*] = seatBelt meta Metaobjects::Metaobject;
        return : String = (seatBelt meta KerML::Feature).qualifiedName
            + "/" + BaseFunctions::ToString(size(everything));
    }
}
