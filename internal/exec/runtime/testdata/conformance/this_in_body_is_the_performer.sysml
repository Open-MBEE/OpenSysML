// `this` written in a behavior body denotes the context occurrence the member
// read `.this` answers. For a performance an object owns that is the owning
// object (Objects.kerml `Object::ownedPerformances` redefines `this default
// that`; Parts.sysml `Part::ownedActions`: "The `this` reference of an
// ownedAction is always its owning Part"), for a subperformance its owner's
// `this` (Performances.kerml `Performance::subperformances` redefines `this
// default (that as Performance).this`), and for the object itself the object
// (Occurrences.kerml `Occurrence::this default self`). So in the part's
// untyped action, its action typed by a def declared beside it, that def's
// subaction, the entry, do and exit actions of its state's definition, and
// its calc and constraint bodies, `this` is the part — read from the body
// (`me`, `who`, `w`), through `self.this` and through the member read alike —
// and `this.id` reaches the part's attribute (`kPlus` reads the calc's result).
package test {
	private import ScalarValues::*;

	action def A {
		ref who = this;
		action sub {
			ref w = this;
		}
	}

	state def M {
		entry action e { ref who = this; }
		do action d { ref who = this; }
		exit action x { ref who = this; }
	}

	part def Rig {
		attribute id : Integer = 7;
		action act {
			attribute mine : Integer = this.id;
			ref me = this;
			ref meSelf = self.this;
		}
		action tact : A;
		state sm : M;
		calc k { this.id + 1 }
		attribute kPlus : Integer = k;
		constraint c { this.id == 7 }
	}

	part rig : Rig;
}
