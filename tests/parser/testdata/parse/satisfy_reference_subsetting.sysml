package SatisfyReferenceSubsetting {
	requirement def R;
	requirement r : R;
	requirement group { requirement nested : R; }
	requirement other : R;
	part v;

	part context {
		satisfy r by v;
		satisfy group.nested by v;
		assert not satisfy r :> other by v;
		satisfy r :>> other;
		satisfy requirement declared :> r by v;
		satisfy requirement subsets r by v;
		requirement plain :> r;
		requirement referencing ::> r;
	}

	verification def V {
		objective {
			verify r;
			verify group.nested :> other;
			verify requirement :> r;
		}
	}
}
