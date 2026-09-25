// A binding end is a connector end (SysML BindingConnectorAsUsage →
// ConnectorEndMember → KerML OwnedReferenceSubsetting), so each end names a
// feature by qualified name or by a chain of qualified names.
package 'Kite Bindings' {
	package R {
		part a;
	}
	part c;

	binding bind R::a = c;
	binding b1 bind R::a = c;

	part 'Kite Environment' {
		part 'Region Earth Surface';
	}
	part 'Kite System' {
		part 'Desert Kite' {
			part 'Kite Wall' {
				attribute 'Wall Height';
			}
		}
	}
	package 'Kite Requirements' {
		attribute 'Minimum Wall Height';
	}

	binding bind 'Kite Environment'::'Region Earth Surface'.'Kite System'::'Desert Kite'.'Kite Wall'.'Wall Height'
		= 'Kite Requirements'::'Minimum Wall Height';
}
