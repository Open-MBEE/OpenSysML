// A kind-less feature may be identified by a short name alone or with a name
// (SysML.xtext DefaultReferenceUsage over Identification), in a namespace, a
// definition body, a calculation body or a nested statement body alike.
package ShortNamedDefaultReferenceUsages {
	private import ScalarValues::*;

	<a> alpha = 1;
	<b> :> alpha = 2;

	part def Tank {
		attribute level : Real;
		<l> limit = 10.0;
		<f> :> limit = 5.0;
		assert constraint bounded {
			<c> ceiling = limit;
			<g> :> ceiling = ceiling + 1.0;
			level < ceiling
		}
	}

	calc def Affine {
		in n : Integer;
		<t> twice = n * 2;
		<o> offset := 1;
		<u> :> twice = 4;
		<v> :> offset := 2;
		twice + offset
	}

	calc def Sum {
		in n : Integer;
		attribute total : Integer = 0;
		attribute i : Integer = 1;
		while i <= n {
			<d> doubled = i * 2;
			<e> :> doubled = doubled;
			assign total := total + doubled;
			assign i := i + 1;
		}
		total
	}

	action def Step {
		<z> z = 1;
		<w> :> z = 2;
	}

	// A KerML-only keyword is an ordinary name in SysML, as a short name or a name.
	<chains> links = 3;
	<s> featured = 1;
	calc def Linked {
		in n : Integer;
		<inv> chains = n;
		chains + 1
	}
}
