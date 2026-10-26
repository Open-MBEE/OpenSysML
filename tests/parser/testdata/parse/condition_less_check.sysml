package test {
	constraint def Empty;
	constraint empty : Empty;
	assert not constraint cneg { assume constraint { false } }
	requirement def Bare;
	requirement bare : Bare;
	requirement outer {
		requirement r0;
		assume r0;
	}
}
