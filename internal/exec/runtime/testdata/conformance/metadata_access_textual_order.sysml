// `.metadata` answers an element's annotations in textual order whichever
// form states them: a `metadata … about` usage written before the element
// comes before its inline `@` annotations, and one written after comes last;
// the element's reflective metaobject follows every annotation (KerML 8.3.4.8.15).
package early {
    metadata test::Review about test::seatBelt {
        reviewer = "ops";
    }
}

package test {
    private import ScalarValues::*;

    metadata def Review {
        attribute reviewer : String = "nobody";
    }

    metadata def Safety {
        attribute level : Integer = 2;
    }

    part def Vehicle;

    part seatBelt : Vehicle {
        @Safety {
            level = 4;
        }
        @Review;
    }

    part def Report {
        attribute annotations [*] = seatBelt.metadata;
        attribute firstReviewer = seatBelt.metadata#(1).reviewer;
        attribute level = seatBelt.metadata#(2).level;
        attribute inlineReviewer = seatBelt.metadata#(3).reviewer;
        attribute lastReviewer = seatBelt.metadata#(4).reviewer;
    }
}

package late {
    metadata test::Review about test::seatBelt {
        reviewer = "qa";
    }
}
