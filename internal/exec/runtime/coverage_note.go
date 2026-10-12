package runtime

import "slices"

// ReasonBlockBodyRepetition is why a run performing a repeated step in a loop or
// if body leaves its coverage observed rather than proved: each performance is
// run as one move, so no interleaving of the performances was ever a schedule.
const ReasonBlockBodyRepetition = "the performances of a repeated step in a loop or if body are each run as one move, so their interleavings were not explored"

// noteCoverage records a reason the run's coverage is narrower than its
// schedules: explore and check then report what they observed, not a proof.
func (c *Context) noteCoverage(reason string) {
	if c.coverageNotes == nil {
		c.coverageNotes = make(map[string]bool)
	}
	c.coverageNotes[reason] = true
}

// coverageReasons lists the distinct reasons recorded, in canonical order.
func (c *Context) coverageReasons() []string {
	out := make([]string, 0, len(c.coverageNotes))
	for reason := range c.coverageNotes {
		out = append(out, reason)
	}
	slices.Sort(out)
	return out
}
