package ReferenceUsage {
	part def Node {
		attribute id : ScalarValues::Integer;
	}

	part def Rig {
		part own : Node {
			:>> id = 1;
		}
		ref part peer : Node;
		ref part chosen : Node = own;
	}
}
