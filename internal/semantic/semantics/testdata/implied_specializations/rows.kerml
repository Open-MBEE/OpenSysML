package K {
	classifier Cf; datatype D; class C; struct S; assoc A { end a : C; end b : C; }
	assoc A3 { end a : C; end b : C; end c : C; } assoc struct AS { end a : C; end b : C; }
	behavior B; function F; predicate P; metaclass M; interaction I;
	class Sub specializes C; struct Both specializes C, S;
	feature f; feature fd : D; feature fc : C; feature fs : S;
	step st; expr ex; bool be; connector cn; binding bd; succession su; flow fl;
	class Owner {
		composite feature oc : C; composite feature os : S; step ost;
	}
	struct SOwner { composite feature so : S; step sst; }
	behavior BOwner { step bst; composite step cst; expr bex; }
}
