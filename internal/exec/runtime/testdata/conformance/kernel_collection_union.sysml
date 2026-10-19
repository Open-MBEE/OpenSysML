package test {
    port def Q;
    // A tracked Kernel collection unions the object with what its subsetting
    // shape features hold: `ownedPorts :> timeEnclosedOccurrences` adds the ports.
    part def Hull {
        port q1 : Q;
        port q2 : Q;
        part pa;
        attribute timeEnclosed : ScalarValues::Integer = self.timeEnclosedOccurrences->SequenceFunctions::size();
        attribute selfFirst : ScalarValues::Boolean = self.timeEnclosedOccurrences#(1) == self;
        attribute portsFollow : ScalarValues::Boolean =
            self.timeEnclosedOccurrences#(2) == q1 and self.timeEnclosedOccurrences#(3) == q2;
        attribute spaceTimeEnclosed : ScalarValues::Integer = self.spaceTimeEnclosedOccurrences->SequenceFunctions::size();
        attribute slices : ScalarValues::Integer = self.timeSlices->SequenceFunctions::size();
        attribute portEnclosed : ScalarValues::Integer = q1.timeEnclosedOccurrences->SequenceFunctions::size();
        attribute partEnclosed : ScalarValues::Integer = pa.timeEnclosedOccurrences->SequenceFunctions::size();
    }
    part hull : Hull;
}
