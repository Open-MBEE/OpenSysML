action def WorkflowAction {
	in ref x : Real;
	out ref y : Real;
	
	// Control flow nodes
	first start;
	done;
	succession first start then done;
	
	// General member (tests Phase B2 fallback)
	attribute counter : Integer;
}
