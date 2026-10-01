// An `exhibit state` usage with an empty body names nothing and exhibits
// itself (SysML v2 §8.3.17, exhibitedState): the machine is its own body, so
// instantiation fails like any machine declaring no initial state, not with
// `classifier behavior names no body`.
package test {
	part def Idle {
		exhibit state idle;
	}
}
