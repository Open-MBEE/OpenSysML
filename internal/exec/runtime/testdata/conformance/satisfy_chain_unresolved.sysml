// A satisfaction assertion whose `by` chain names a feature its operand does not
// hold: the assertion has no subject, and the diagnostic names the chain as
// written rather than its last segment alone.
package test {
    private import ScalarValues::Real;

    part def Part {
        attribute mass : Real = 5.0;
    }

    requirement def MassReq {
        subject subj : Part;
        require constraint { subj.mass < 10.0 }
    }

    part def Config {
        part child : Part;
    }

    part config : Config;

    requirement r : MassReq;
    satisfy r by config.nope;
}
