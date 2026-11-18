// An `exhibit state` with an empty body exhibits itself (SysML v2 §8.3.17) and,
// declaring no substate, is active in itself (§7.18.1).
package test {
	part def Idle {
		exhibit state idle;
	}
}
