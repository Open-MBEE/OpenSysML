// A three-element specialization cycle: a check can catch the direct shapes and
// miss the transitive one, so it is probed separately (F4).
package 'Specialization Cycle Three Probe' {
	part def C1 specializes C2;
	part def C2 specializes C3;
	part def C3 specializes C1;
}
