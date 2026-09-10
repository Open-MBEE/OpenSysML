package repl

import "testing"

// A model whose element names hold characters no basic name can: they are
// declared quoted, and must be typed quoted.
const quotedNamesModel = `package T {
	part def Rocket;
	individual part def 'SA-506' :> Rocket;
	requirement def <'HLR-R001'> 'Hi Level';
	calc def 'Delta-V' { return : ScalarValues::Real = 1.0; }
	state def 'Main-Loop';
}`

const quotingHint = "did you mean T::'SA-506'? Names containing '-' must be quoted."

// An unquoted spelling of a quoted name is read as far as the notation reads
// it, and the failure names the quoted declaration that spelling starts.
func TestUnquotedNameIsPointedAtItsQuotedDeclaration(t *testing.T) {
	t.Run("%features", func(t *testing.T) {
		s := submitted(t, quotedNamesModel)
		got := run(t, s, "%features T::SA-506")
		wants(t, got, `error: "T::SA-506" is not an object reference`, quotingHint)
	})

	t.Run("%features of a path under the name", func(t *testing.T) {
		s := submitted(t, quotedNamesModel)
		wants(t, run(t, s, "%features T::SA-506.mass"), "is not an object reference", quotingHint)
	})

	t.Run("%instantiate", func(t *testing.T) {
		s := submitted(t, quotedNamesModel)
		wants(t, run(t, s, "%instantiate T::SA"), "error: unresolved reference: T::SA — "+quotingHint)
		wants(t, run(t, s, "%instantiate SA"), "error: unresolved reference: SA — "+quotingHint)
	})

	t.Run("%eval", func(t *testing.T) {
		s := submitted(t, quotedNamesModel)
		wants(t, run(t, s, "%eval T::SA-506"), "error:", "unresolved reference: T::SA — "+quotingHint)
		wants(t, run(t, s, "%eval SA-506"),
			"unresolved reference: SA — did you mean 'SA-506'? Names containing '-' must be quoted.")
	})

	t.Run("%calc", func(t *testing.T) {
		s := submitted(t, quotedNamesModel)
		wants(t, run(t, s, "%calc T::Delta 1"),
			"error: unresolved reference: T::Delta — did you mean T::'Delta-V'? Names containing '-' must be quoted.")
	})

	// Offered only of the kinds the command acts on: a part def is no state.
	t.Run("%state", func(t *testing.T) {
		s := submitted(t, quotedNamesModel)
		wants(t, run(t, s, "%state T::Main"),
			"unresolved reference: T::Main — did you mean T::'Main-Loop'? Names containing '-' must be quoted.")
		got := run(t, s, "%state T::SA")
		wants(t, got, "unresolved reference: T::SA")
		rejects(t, got, "must be quoted")
	})
}

// The quoted spelling the hint offers is the one that works, and an expression
// that subtracts remains one: `SA-506` is not read as a name anywhere.
func TestQuotedSpellingWorksAndSubtractionStillSubtracts(t *testing.T) {
	s := submitted(t, quotedNamesModel)
	wants(t, run(t, s, "%instantiate T::'SA-506'"), "✓ Created instance of T::'SA-506'")
	wants(t, run(t, s, "%features T::'SA-506'"), "Instance: T::'SA-506' (ID: 1)")
	wants(t, run(t, s, "%calc T::'Delta-V'"), "1.0")
	wants(t, run(t, s, "%eval 506-6"), "= 500")
	rejects(t, run(t, s, "%eval 506-6"), "must be quoted")
}

// A short name is offered under the spelling that was matched, and a name
// %features would not read as typed is echoed as the notation writes it.
func TestUnquotedShortNameAndInstantiateEcho(t *testing.T) {
	s := submitted(t, quotedNamesModel)
	wants(t, run(t, s, "%instantiate T::HLR"),
		"error: unresolved reference: T::HLR — did you mean T::'HLR-R001'? Names containing '-' must be quoted.")
	wants(t, run(t, s, "%instantiate HLR"),
		"error: unresolved reference: HLR — did you mean T::'HLR-R001'? Names containing '-' must be quoted.")
	wants(t, run(t, s, "%instantiate T::'HLR-R001'"), "Use %features T::'HLR-R001' to inspect")
	wants(t, run(t, s, "%instantiate T::SA-506"), "Use %features T::'SA-506' to inspect")
	rejects(t, run(t, s, "%instantiate T::SA-506"), "Use %features T::SA-506 to inspect")
}

// A name that resolves to nothing quoted either is reported as before: the
// quoting rule is stated only where a quoted declaration is what it starts.
func TestNoQuotingHintWithoutAQuotedDeclaration(t *testing.T) {
	s := submitted(t, `package T { part def Rocket; part def SA_506 :> Rocket; part def SATURN; }`)
	got := run(t, s, "%instantiate T::SA")
	wants(t, got, "error: unresolved reference: T::SA")
	rejects(t, got, "must be quoted")
}
