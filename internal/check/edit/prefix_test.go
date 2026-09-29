package edit

import (
	"strings"
	"testing"
)

func TestAddMetadataPrefixWritesInTheUsagePrefixSlot(t *testing.T) {
	tests := []struct {
		name, source, target, want string
	}{
		{
			name:   "definition visibility",
			source: "metadata def M;\npublic part def A;\n",
			target: "A",
			want:   "metadata def M;\npublic #M part def A;\n",
		},
		{
			name:   "abstract",
			source: "metadata def M;\nabstract part def A;\n",
			target: "A",
			want:   "metadata def M;\nabstract #M part def A;\n",
		},
		{
			name:   "variation",
			source: "metadata def M;\nvariation part def A;\n",
			target: "A",
			want:   "metadata def M;\nvariation #M part def A;\n",
		},
		{
			name:   "individual definition",
			source: "metadata def M;\nindividual part def A;\n",
			target: "A",
			want:   "metadata def M;\nindividual #M part def A;\n",
		},
		{
			name:   "existing prefix",
			source: "metadata def M;\nmetadata def N;\n#N part def A;\n",
			target: "A",
			want:   "metadata def M;\nmetadata def N;\n#N #M part def A;\n",
		},
		{
			name:   "direction",
			source: "metadata def M;\npart def T;\nin attribute x : T;\n",
			target: "x",
			want:   "metadata def M;\npart def T;\nin #M attribute x : T;\n",
		},
		{
			name:   "derived",
			source: "metadata def M;\nattribute def T;\noccurrence def C { derived attribute x : T; }\n",
			target: "C::x",
			want:   "metadata def M;\nattribute def T;\noccurrence def C { derived #M attribute x : T; }\n",
		},
		{
			name:   "constant",
			source: "metadata def M;\nattribute def T;\noccurrence def C { constant attribute x : T; }\n",
			target: "C::x",
			want:   "metadata def M;\nattribute def T;\noccurrence def C { constant #M attribute x : T; }\n",
		},
		{
			name:   "ref part",
			source: "metadata def M;\npart def T;\nref part x : T;\n",
			target: "x",
			want:   "metadata def M;\npart def T;\nref #M part x : T;\n",
		},
		{
			name:   "ref keywordless usage",
			source: "metadata def M;\npart def T;\nref x : T;\n",
			target: "x",
			want:   "metadata def M;\npart def T;\nref #M x : T;\n",
		},
		{
			name:   "keywordless usage",
			source: "metadata def M;\npart def T;\nx : T;\n",
			target: "x",
			want:   "metadata def M;\npart def T;\n#M x : T;\n",
		},
		{
			name: "combined modifiers",
			source: "metadata def M;\nattribute def T;\n" +
				"action def C { in derived abstract constant ref attribute x : T; }\n",
			target: "C::x",
			want: "metadata def M;\nattribute def T;\n" +
				"action def C { in derived abstract constant ref #M attribute x : T; }\n",
		},
		{
			name:   "end in connection definition",
			source: "metadata def M;\npart def T;\nconnection def C { end part a : T; end part b : T; }\n",
			target: "C::a",
			want:   "metadata def M;\npart def T;\nconnection def C { end #M part a : T; end part b : T; }\n",
		},
		{
			name:   "end with cross-feature",
			source: "metadata def M;\npart def T;\nconnection def C { end [1] part a : T; end part b : T; }\n",
			target: "C::a",
			want:   "metadata def M;\npart def T;\nconnection def C { end [1] #M part a : T; end part b : T; }\n",
		},
		{
			name:   "snapshot",
			source: "metadata def M;\npart def T;\noccurrence def O;\noccurrence car : O { snapshot part x : T; }\n",
			target: "car::x",
			want:   "metadata def M;\npart def T;\noccurrence def O;\noccurrence car : O { snapshot #M part x : T; }\n",
		},
		{
			name:   "variant",
			source: "metadata def M;\npart def T;\nvariation part def V { variant part x : T; }\n",
			target: "V::x",
			want:   "metadata def M;\npart def T;\nvariation part def V { variant #M part x : T; }\n",
		},
		{
			name:   "fork",
			source: "metadata def M;\naction def A { action a { fork f; } }\n",
			target: "A::a::f",
			want:   "metadata def M;\naction def A { action a { #M fork f; } }\n",
		},
		{
			name:   "join",
			source: "metadata def M;\naction def A { action a { join j; } }\n",
			target: "A::a::j",
			want:   "metadata def M;\naction def A { action a { #M join j; } }\n",
		},
		{
			name:   "merge",
			source: "metadata def M;\naction def A { action a { merge m; } }\n",
			target: "A::a::m",
			want:   "metadata def M;\naction def A { action a { #M merge m; } }\n",
		},
		{
			name:   "decide",
			source: "metadata def M;\naction def A { action a { decide d; } }\n",
			target: "A::a::d",
			want:   "metadata def M;\naction def A { action a { #M decide d; } }\n",
		},
		{
			name:   "subject",
			source: "metadata def M;\npart def T;\nrequirement def R { subject s : T; }\n",
			target: "R::s",
			want:   "metadata def M;\npart def T;\nrequirement def R { subject #M s : T; }\n",
		},
		{
			name:   "actor",
			source: "metadata def M;\npart def T;\nrequirement def R { actor a : T; }\n",
			target: "R::a",
			want:   "metadata def M;\npart def T;\nrequirement def R { actor #M a : T; }\n",
		},
		{
			name:   "stakeholder",
			source: "metadata def M;\npart def T;\nrequirement def R { stakeholder s : T; }\n",
			target: "R::s",
			want:   "metadata def M;\npart def T;\nrequirement def R { stakeholder #M s : T; }\n",
		},
		{
			name:   "objective",
			source: "metadata def M;\nverification def V { objective o; }\n",
			target: "V::o",
			want:   "metadata def M;\nverification def V { objective #M o; }\n",
		},
		{
			name:   "assume",
			source: "metadata def M;\nrequirement def R { assume constraint c; }\n",
			target: "R::c",
			want:   "metadata def M;\nrequirement def R { assume #M constraint c; }\n",
		},
		{
			name:   "require",
			source: "metadata def M;\nrequirement def R { require constraint c; }\n",
			target: "R::c",
			want:   "metadata def M;\nrequirement def R { require #M constraint c; }\n",
		},
		{
			name:   "then",
			source: "metadata def M;\naction def A { action before; then action a; }\n",
			target: "A::a",
			want:   "metadata def M;\naction def A { action before; then #M action a; }\n",
		},
		{
			name:   "return",
			source: "metadata def M;\npart def T;\ncalc def C { return attribute r : T; }\n",
			target: "C::r",
			want:   "metadata def M;\npart def T;\ncalc def C { return #M attribute r : T; }\n",
		},
		{
			name:   "verify",
			source: "metadata def M;\nrequirement def R;\nrequirement r :> R;\nverification def V { objective o { verify requirement rr :> r; } }\n",
			target: "V::o::rr",
			want:   "metadata def M;\nrequirement def R;\nrequirement r :> R;\nverification def V { objective o { verify #M requirement rr :> r; } }\n",
		},
		{
			name:   "frame",
			source: "metadata def M;\nrequirement def R { frame concern c; }\n",
			target: "R::c",
			want:   "metadata def M;\nrequirement def R { frame #M concern c; }\n",
		},
		{
			name:   "render",
			source: "metadata def M;\nview def V { render rendering x; }\n",
			target: "V::x",
			want:   "metadata def M;\nview def V { render #M rendering x; }\n",
		},
		{
			name:   "library package",
			source: "metadata def M;\nlibrary package Q;\n",
			target: "Q",
			want:   "metadata def M;\nlibrary #M package Q;\n",
		},
		{
			name:   "standard library package",
			source: "metadata def M;\nstandard library package Q;\n",
			target: "Q",
			want:   "metadata def M;\nstandard library #M package Q;\n",
		},
		{
			name:   "plain package",
			source: "metadata def M;\npackage Q;\n",
			target: "Q",
			want:   "metadata def M;\n#M package Q;\n",
		},
		{
			name:   "comment between modifiers",
			source: "metadata def M;\npublic /* retain */ abstract part def A;\n",
			target: "A",
			want:   "metadata def M;\npublic /* retain */ abstract #M part def A;\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := loadContent(t, "metadata-prefix.sysml", tt.source)
			requireClean(t, model)
			result, err := Apply(model, []Operation{AddMetadataPrefix(tt.target, "M")})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := string(result.Content); got != tt.want {
				t.Fatalf("content = %q, want %q", got, tt.want)
			}
			requireClean(t, loadContent(t, "metadata-prefix.sysml", string(result.Content)))
		})
	}
}

func TestAddMetadataPrefixRefusals(t *testing.T) {
	tests := []struct {
		name, file, source, target, metadataType string
		want                                     Failure
	}{
		{
			name: "KerML source", file: "metadata-prefix.kerml",
			source: "metadata def M;\npart def A;\n", target: "A", metadataType: "M",
			want: FailureIllegalKind,
		},
		{
			name: "unresolved metadata type", file: "metadata-prefix.sysml",
			source: "part def A;\n", target: "A", metadataType: "Missing",
			want: FailureInvalidValue,
		},
		{
			name: "metadata type outside target scope", file: "metadata-prefix.sysml",
			source: "package Types { metadata def M; }\npackage P { part def A; }\n",
			target: "P::A", metadataType: "M", want: FailureInvalidValue,
		},
		{
			name: "not a metadata definition", file: "metadata-prefix.sysml",
			source: "part def A;\npart def M;\n", target: "A", metadataType: "M",
			want: FailureInvalidValue,
		},
		{
			name: "KerML metaclass", file: "metadata-prefix.sysml",
			source: "part def A;\n", target: "A", metadataType: "Metaobjects::Metaobject",
			want: FailureInvalidValue,
		},
		{
			name: "qualified duplicate", file: "metadata-prefix.sysml",
			source: "package P { metadata def M; #P::M part def A; }\n",
			target: "P::A", metadataType: "M", want: FailureInvalidValue,
		},
		{
			name: "entry action", file: "metadata-prefix.sysml",
			source: "state def S { entry action boot; }\n",
			target: "S::boot", metadataType: "M", want: FailureIllegalKind,
		},
		{
			name: "transition", file: "metadata-prefix.sysml",
			source: "metadata def M;\nstate def S { state a; state b; transition t first a then b; }\n",
			target: "S::t", metadataType: "M", want: FailureIllegalKind,
		},
		{
			name: "alias target", file: "metadata-prefix.sysml",
			source: "part def A;\nalias B for A;\nmetadata def M;\n",
			target: "B", metadataType: "M", want: FailureIllegalKind,
		},
		{
			name: "imported name is not a declaration target", file: "metadata-prefix.sysml",
			source: "package P { part def A; }\npackage Q { private import P::A; }\n",
			target: "Q::A", metadataType: "M", want: FailureUnknownTarget,
		},
		{
			name: "unknown target", file: "metadata-prefix.sysml",
			source: "metadata def M;\n", target: "missing", metadataType: "M",
			want: FailureUnknownTarget,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := loadContent(t, tt.file, tt.source)
			err := addFailure(t, model, AddMetadataPrefix(tt.target, tt.metadataType), tt.want)
			if tt.name == "unresolved metadata type" &&
				!strings.Contains(err.Message, `metadata type "Missing" does not resolve from A`) {
				t.Fatalf("message = %q, want unresolved metadata type context", err.Message)
			}
			if tt.name == "KerML metaclass" &&
				!strings.Contains(err.Message, `"Metaobjects::Metaobject" is a metaclass, not a metadata definition`) {
				t.Fatalf("message = %q, want resolved metaclass refusal", err.Message)
			}
			if tt.name == "metadata type outside target scope" &&
				!strings.Contains(err.Message, `metadata type "M" does not resolve from P::A`) {
				t.Fatalf("message = %q, want target-scope resolution failure", err.Message)
			}
		})
	}
}

func TestAddMetadataPrefixRejectsUnqualifiedDuplicate(t *testing.T) {
	const src = "package P { metadata def M; #M part def A; }\n"
	model := loadContent(t, "metadata-prefix.sysml", src)
	requireClean(t, model)
	err := addFailure(t, model, AddMetadataPrefix("P::A", "P::M"), FailureInvalidValue)
	if !strings.Contains(err.Message, `already carries #P::M`) {
		t.Fatalf("message = %q, want duplicate metadata type", err.Message)
	}
}
