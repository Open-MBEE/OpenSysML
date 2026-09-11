package migrate

import "testing"

func TestStandardNamespaces(t *testing.T) {
	for ns, want := range map[string]bool{
		"http://www.omg.org/spec/SysML/20181001/SysML":           true,
		"http://schema.omg.org/spec/SysML/1.3":                   true,
		"http://www.omg.org/spec/UML/20161101/StandardProfile":   true,
		"http://www.eclipse.org/uml2/5.0.0/UML/Profile/Standard": true,
		"http://www.eclipse.org/papyrus/sysml/1.6/SysML/Blocks":  true,
		"http://www.eclipse.org/papyrus/acme/profile":            false,
		"http://www.eclipse.org/papyrus/acme/sysml/profile":      false,
		"http://www.eclipse.org/emf/2002/Ecore":                  false,
		"https://profiles.example/UML/Profile/Standard/acme":     false,
		"http://www.example.com/omg.org/spec/SysML/lookalike":    false,
		"http://omg.org.example.com/spec/SysML/lookalike":        false,
		"http://www.example.com/tool/customization/SysML":        false,
		"http://www.eclipse.org/uml2/5.0.0/Types":                false,
		"": false,
	} {
		if got := isStandardNamespace(ns); got != want {
			t.Errorf("isStandardNamespace(%q) = %v, want %v", ns, got, want)
		}
	}
}
