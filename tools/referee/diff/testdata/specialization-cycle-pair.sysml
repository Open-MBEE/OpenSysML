// The two-element specialization cycle of testdata/passes/constraints.sysml,
// isolated from that file's bare `import`, which breaks the pilot's parse (F4).
package 'Specialization Cycle Pair Probe' {
	part def B1 specializes B2;
	part def B2 specializes B1;
}
