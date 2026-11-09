package repl

import "testing"

// subjectChainModel states one condition twice: r3 through a value derived from the
// subject, r4 directly over the subject; neither can hold.
const subjectChainModel = `
package SubjectChain {
    private import ScalarValues::*;
    part def Loan { attribute amount : Real; }

    abstract requirement def R {
        subject s : Loan;
        attribute ok : Boolean;
        require constraint { ok }
    }
    requirement r3 : R {
        attribute :>> ok = s.amount > 2.0 and s.amount < 1.0;
    }
    requirement r4 {
        subject s : Loan;
        require constraint { s.amount > 2.0 and s.amount < 1.0 }
    }
}
`

// TestCheckDerivesAValueThroughTheSubject: a value declared through the subject, a parameter
// (SysML v2 §7.21.2) no object is held for, is asserted as its equation, so r3 answers as r4.
func TestCheckDerivesAValueThroughTheSubject(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, subjectChainModel)
	wants(t, run(t, s, "%check SubjectChain::r3"), "✗ Requirement r3 is unsatisfiable")
	wants(t, run(t, s, "%explain SubjectChain::r3"),
		"defining equation: `SubjectChain::r3::ok == s.amount > 2.0 and s.amount < 1.0, declared by the model`",
		"required condition: `ok`")
	wants(t, run(t, s, "%check SubjectChain::r4"), "✗ Requirement r4 is unsatisfiable")
}
