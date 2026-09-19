package Demo {
    private import ShapeItems::*;
    private import SI::*;
    part box : Box {
        :>> length = 2 [m];
        :>> width = 1 [m];
        :>> height = 1 [m];
    }
    requirement def MassLimit {
        doc /* Keep the mass under the limit. */
        attribute limit : ISQ::MassValue = 10 [kg];
    }
    requirement massLimit : MassLimit {
        subject vehicle : Parts::Part = box;
    }
}
