// A braced state behavior block is one anonymous action usage: `entry { … }`
// parses to the tree `entry action { … }` does, likewise `do`, `exit` and a
// transition's `do { … }`. Each braced form is written beside its `action`
// spelling. Declarations inside a block belong to the block.
package BracedBlocks {
	state def Machine {
		attribute c : Integer = 0;
		item x;
		part operator;
		state a {
			entry {
				attribute k : Integer = 2;
				assign c := c + k;
				send x to operator;
			}
			do { assign c := c + 1; assign c := c + 2; }
			exit { }
		}
		state b {
			entry action {
				attribute k : Integer = 2;
				assign c := c + k;
				send x to operator;
			}
			do action { assign c := c + 1; assign c := c + 2; }
			exit action { }
		}
		state c;
		entry; then a;
		transition first a accept x do { assign c := 0; send x to operator; } then b;
		transition first b accept x do action { assign c := 0; send x to operator; } then a;
		transition first a accept when true do { } then c;
		transition first b accept when true do action { } then c;
	}
}
