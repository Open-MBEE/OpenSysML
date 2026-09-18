// A binding end may carry an end multiplicity before the feature it names
// (SysML.xtext:1020 BindingConnectorAsUsage, `bind [0..*] base.edges = [0..*] be;`
// throughout the Geometry library).
package 'Indexed Bindings' {
	part def Shape {
		part base {
			part edges[0..*];
		}
		part be[0..*];

		bind [0..*] base.edges = [0..*] be;
		binding [1] bind [0..*] base.edges = [0..*] be;
	}
}
