// An action body resolves a name against the scope it was written in, so it
// reaches the members of the package enclosing it: `pkgLevel` is declared beside
// the part owning the action, not in the action. The action is owned by a part,
// which the case reaches by naming its qualified path as the entry behavior.
package test {
	public import ScalarValues::*;

	attribute pkgLevel : Real = 7.0;

	part owner {
		action byPkgRef {
			attribute r : Real = 0.0;

			first start;

			action bump {
				assign r := pkgLevel + 1.0;
			}

			done;

			succession first start then bump;
			succession first bump then done;
		}
	}
}
