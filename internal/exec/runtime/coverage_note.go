package runtime

import "slices"

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
