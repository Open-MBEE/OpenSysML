// Each binding end is a ConnectorEnd (SysML.xtext BindingConnectorAsUsage →
// ConnectorEndMember): an optional end multiplicity, an optional end name with
// its ReferencesKeyword, and the referenced feature. The end name belongs to the
// end, never to the binding, so `bind e1 ::> a` names no connector.
package 'Binding Connector Ends' {
	part def AB;
	part def Tank {
		attribute a;
		attribute b;
		part p {
			attribute x;
		}
		part q {
			attribute y;
		}

		bind a = b;
		bind [1] a = [1] b;
		bind e1 ::> a = e2 references b;
		bind [0..1] e3 ::> a = [0..1] e4 ::> b;
		bind e5 ::> a = b;
		bind a = e6 ::> b;

		binding x : AB bind a = b;
		binding [1] bind a = b;
		binding : AB bind a = b;
		binding y bind a = b;
		binding z : AB [1] bind e7 ::> a = e8 ::> b;
		binding w bind a = b {
			doc /* A binding may carry a body. */
		}

		bind p.x = q.y;
		bind Tank::a = Tank::b;
		bind $::'Binding Connector Ends'::Tank::a = b;
	}
}
