part def RedefinedValues {
	// `redefines <target> = <value>` reads the target by parsing it and rewinding
	// when no '=' follows, so the target's length is not capped.
	redefines value = 100;
	redefines parent.value = 100;
	redefines outer.middle.inner.leaf.deeper.deepest.last = 1;
	redefines Outer::Middle::inner.leaf = 2;
}
