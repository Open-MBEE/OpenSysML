// Another fragment of Cases: dynamic multiplicity, joined redefinitions and C# contextual keywords.
package Cases {
    private import ScalarValues::*;

    attribute readingCount : Natural = 3;

    part def DynamicReadings {
        attribute readings : Real[0..readingCount] = (1.0, 2.0, 3.0);
    }

    part def BroadReading {
        attribute reading : Real[0..*];
    }

    part def NarrowReading :> BroadReading {
        attribute reading : Integer[0..1] redefines BroadReading::reading;
    }

    part def JoinedReading :> BroadReading, NarrowReading {
        attribute reading redefines BroadReading::reading, NarrowReading::reading;
    }

    part def 'record';

    package 'file' {
        part def 'required';
        part def 'scoped';
        part def '__arglist';
    }
}
