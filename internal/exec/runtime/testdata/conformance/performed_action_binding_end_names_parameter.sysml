// A binding end written at a node of a performed action is a statement of that
// action's body: a name there resolves to the action's own parameter before a
// same-named feature of the part performing it. A parameter admitting no value
// and given none is read as empty, not as the part's value.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Host {
		attribute level : Integer = 7;
		attribute seen : Integer = 0;

		perform action relaying {
			in level : Integer[0..1];

			first start;

			action noting {
				in n : Integer[0..1];
				assign seen := size(n);
			}
			bind noting.n = level;

			done;

			succession first start then noting;
			succession first noting then done;
		}
	}
}
