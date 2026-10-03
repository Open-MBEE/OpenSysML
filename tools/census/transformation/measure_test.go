package transformation

import (
	"os"
	"path/filepath"
	"testing"
)

// The qualified scope corpus: ControlFlows exercising every qualifier shape,
// plus a sysml stereotype application and a nested same-type element.
const qualifiedXMI = `<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20131001" xmlns:syspar="http://www.omg.org/spec/SysML/20181001">
<packagedElement xmi:type="uml:ControlFlow" xmi:id="a" weight="1"/>
<packagedElement xmi:type="uml:ControlFlow" xmi:id="b"><weight href="w"/></packagedElement>
<packagedElement xmi:type="uml:ControlFlow" xmi:id="c" weight="2"><weight href="w"/></packagedElement>
<packagedElement xmi:type="uml:ControlFlow" xmi:id="d">
  <packagedElement xmi:type="uml:ControlFlow" xmi:id="e" weight="9"/>
</packagedElement>
<packagedElement xmi:type="uml:ControlFlow" xmi:id="f" direction="return"/>
<packagedElement xmi:type="uml:ControlFlow" xmi:id="g"><direction>in</direction></packagedElement>
<syspar:Block xmi:id="s1" isEncapsulated="true"/>
<syspar:Block xmi:id="s2" isEncapsulated="false"/>
<syspar:Block xmi:id="s3"/>
</xmi:XMI>`

func countTestTokens(t *testing.T, tokens ...string) map[string]int {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c.xmi")
	if err := os.WriteFile(path, []byte(qualifiedXMI), 0o644); err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, tok := range tokens {
		set[tok] = true
	}
	counts, err := countTokens(path, set)
	if err != nil {
		t.Fatal(err)
	}
	return counts
}

func TestCountTokensQualifierAttributePresence(t *testing.T) {
	counts := countTestTokens(t, "uml:ControlFlow", "uml:ControlFlow[weight]")
	if counts["uml:ControlFlow"] != 7 {
		t.Fatalf("uml:ControlFlow = %d, want 7", counts["uml:ControlFlow"])
	}
	// a (attribute), b (child), c (both), e (nested) carry weight; d, f, g do not.
	if counts["uml:ControlFlow[weight]"] != 4 {
		t.Fatalf("uml:ControlFlow[weight] = %d, want 4", counts["uml:ControlFlow[weight]"])
	}
}

func TestCountTokensQualifierAttributeValue(t *testing.T) {
	counts := countTestTokens(t, "uml:ControlFlow[direction=return]", "uml:ControlFlow[direction=in]")
	// f matches exactly; g's direction is only a child, so it matches neither.
	if counts["uml:ControlFlow[direction=return]"] != 1 {
		t.Fatalf("direction=return = %d, want 1", counts["uml:ControlFlow[direction=return]"])
	}
	if counts["uml:ControlFlow[direction=in]"] != 0 {
		t.Fatalf("direction=in = %d, want 0", counts["uml:ControlFlow[direction=in]"])
	}
}

func TestCountTokensQualifierNoLeakToParent(t *testing.T) {
	counts := countTestTokens(t, "uml:ControlFlow[weight=9]", "uml:ControlFlow[direction]")
	// Only e carries weight=9; the qualifier flag of a nested ControlFlow must
	// not satisfy d.
	if counts["uml:ControlFlow[weight=9]"] != 1 {
		t.Fatalf("weight=9 = %d, want 1", counts["uml:ControlFlow[weight=9]"])
	}
	// e's presence inside d does not give d a direction child.
	if counts["uml:ControlFlow[direction]"] != 2 {
		t.Fatalf("direction = %d, want 2 (f attribute, g child)", counts["uml:ControlFlow[direction]"])
	}
}

func TestCountTokensQualifierSysml(t *testing.T) {
	counts := countTestTokens(t, "sysml:Block", "sysml:Block[isEncapsulated=true]", "sysml:Block[isEncapsulated]")
	if counts["sysml:Block"] != 3 {
		t.Fatalf("sysml:Block = %d, want 3", counts["sysml:Block"])
	}
	if counts["sysml:Block[isEncapsulated=true]"] != 1 {
		t.Fatalf("isEncapsulated=true = %d, want 1", counts["sysml:Block[isEncapsulated=true]"])
	}
	if counts["sysml:Block[isEncapsulated]"] != 2 {
		t.Fatalf("isEncapsulated = %d, want 2", counts["sysml:Block[isEncapsulated]"])
	}
}

func TestScopeTokenGrammar(t *testing.T) {
	for _, good := range []string{
		"uml:Class", "sysml:Block", "uml:ControlFlow[weight]",
		"uml:Parameter[direction=return]", "sysml:Block[isEncapsulated=true]",
	} {
		if !scopeToken.MatchString(good) {
			t.Errorf("scope token %q rejected", good)
		}
	}
	for _, bad := range []string{
		"uml:", "xmi:Class", "uml:ControlFlow[weight", "uml:ControlFlow[weight]]",
		"uml:ControlFlow[=x]", "uml:ControlFlow[a b]", "uml:ControlFlow[]",
	} {
		if scopeToken.MatchString(bad) {
			t.Errorf("scope token %q accepted", bad)
		}
	}
}
