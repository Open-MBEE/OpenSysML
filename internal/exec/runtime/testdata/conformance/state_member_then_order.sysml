package MemberThenOrderTest {
	// A `then` attached to a state declares a completion transition, so the
	// machine follows it rather than declaration order: `fourth` is entered after
	// `second` only because of the keyword between them.
	state Sequenced {
		entry; then start;
		state start;

		state second;
		then state fourth;

		state third;

		succession first start then second;
		succession first fourth then third;
		succession first third then done;
	}
}
