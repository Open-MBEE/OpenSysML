// A KerML binding states a FeatureDeclaration followed by `of end = end`, or
// omits the declaration and states the ends directly, `of` optional
// (KerML.xtext BindingConnectorDeclaration). Each end is a ConnectorEnd with an
// optional multiplicity and an optional `Name ReferencesKeyword` end name.
package 'Binding Connector Ends' {
	classifier AB;
	classifier Tank {
		feature a;
		feature b;
		feature p {
			feature x;
		}
		feature q {
			feature y;
		}

		binding a = b;
		binding of a = b;
		binding x of a = b;
		binding all of a = b;
		binding all a = b;
		binding [1] a = b;
		binding of [1] a = [1] b;
		binding of e1 ::> a = e2 ::> b;
		binding e3 references a = e4 references b;
		binding x2 of e5 references a = e6 references b;
		binding [1] e7 ::> a = [1] e8 ::> b;
		binding x3 : AB [1] of a = b;
		binding x4 {
			doc /* A body without ends. */
		}
		binding {
			doc /* An anonymous body without ends. */
		}
		binding x5;
		binding p.x = q.y;
		binding Tank::a = Tank::b;
		binding $::'Binding Connector Ends'::Tank::a = b;
	}
}
