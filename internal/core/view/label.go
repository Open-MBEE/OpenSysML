package view

// labelHead is the first line of a node's diagram label: its name, followed by
// " : Type" for a typed usage. A node with no name leads with its kind instead.
func labelHead(node *Node) string {
	if node.Name == "" {
		return node.Kind
	}
	if node.Type == "" {
		return node.Name
	}
	return node.Name + " : " + node.Type
}

// labelLines is a node's diagram label in the graphical notation's order: the
// head, the kind in guillemets when the head is a name, then the notes.
func labelLines(node *Node) []string {
	lines := []string{labelHead(node)}
	if node.Name != "" {
		lines = append(lines, "«"+node.Kind+"»")
	}
	if node.Detail != "" {
		lines = append(lines, node.Detail)
	}
	return lines
}
