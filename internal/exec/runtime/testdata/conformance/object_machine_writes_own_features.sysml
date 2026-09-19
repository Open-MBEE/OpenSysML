// A machine an object exhibits reads and writes that object's own feature
// values: the two nested nodes run the same machine over different `id` values,
// so each `seen` is written from the node performing it.
package test {
	private import ScalarValues::*;

	part def Node {
		attribute id : Integer = 0;
		attribute seen : Integer = 0;

		exhibit state marking {
			entry; then ready;

			state ready {
				entry action mark {
					assign seen := seen + id;
				}
			}
		}
	}

	part def Pair {
		part a : Node {
			attribute :>> id = 1;
		}
		part b : Node {
			attribute :>> id = 2;
		}
	}
}
