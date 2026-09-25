// A behaviour an object performs runs as that object, however deeply nested: the
// accept two performances down takes the message addressed to the part by name.
package SendIdentityPerformed {
	action def Inner {
		first start;

		action reader accept n : Integer;

		done;

		succession first start then reader;
		succession first reader then done;
	}

	action def Outer {
		first start;

		action mid : Inner;

		done;

		succession first start then mid;
		succession first mid then done;
	}

	part def Node {
		state Listen {
			entry; then start;
			state start;

			state waiting {
				entry send 7 to worker;
			}
			state heard;

			succession first start then waiting;

			transition first waiting then heard do perform Outer;
		}
	}

	part worker : Node;
}
