// `.metadata` (KerML 8.3.4.8.15 MetadataAccessExpression) reads the metadata
// annotating an element: one object per annotation, in the order stated,
// carrying the values its body binds over the defaults its type declares,
// followed by the element's own reflective metaobject.
package test {
    private import ScalarValues::*;

    metadata def Safety {
        attribute isMandatory : Boolean;
        attribute level : Integer = 2;
    }

    metadata def Legacy {
        attribute supersededBy : String = "none";
    }

    part def Vehicle;

    part seatBelt : Vehicle {
        @Safety {
            isMandatory = true;
            level = 4;
        }
        @Legacy;
    }

    part airbag : Vehicle {
        @Safety;
    }

    part chassis : Vehicle;

    part def Report {
        attribute annotations [*] = seatBelt.metadata;
        attribute mandatory = seatBelt.metadata#(1).isMandatory;
        attribute level = seatBelt.metadata#(1).level;
        attribute superseded = seatBelt.metadata#(2).supersededBy;
        attribute defaulted = airbag.metadata#(1).level;
        attribute none [*] = chassis.metadata;
    }
}
