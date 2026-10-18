// Demo::Part is declared twice, so an action id through it names no one
// declaration and is not found, rather than answered by either duplicate.
package Demo {
	part def A {
		action run { first start; then done; }
	}
	part def B {
		action run { first start; then done; }
	}
	part def Part :> A;
	part def Part :> B;
}
