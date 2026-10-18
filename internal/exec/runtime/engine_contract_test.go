package runtime

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/enginecontract"
)

// The extension packages the vendored library carries and the runtime function
// registry cannot drift: every registry entry under an extension package is a
// manifest `function` entry with identical parameters in order, and every
// manifest `function` entry in a package the registry implements is registered.
func TestEngineContractMatchesTheRuntimeRegistry(t *testing.T) {
	src := libs.DefaultSource()
	manifest, err := enginecontract.Load(src)
	if err != nil {
		t.Fatalf("%v", err)
	}
	packages, err := enginecontract.Packages(src)
	if err != nil {
		t.Fatalf("%v", err)
	}
	byName := manifest.ByName()

	update := "update engine-contract.json upstream in Open-MBEE/OpenSysML-Extensions-Library, bump scripts/extension-libraries-pin.sh, and run scripts/sync-extension-libraries.sh"

	registryPackages := map[string]bool{}
	for name := range libraryFunctions {
		pkg, _, _ := strings.Cut(name, "::")
		registryPackages[pkg] = true
	}

	for name, fn := range libraryFunctions {
		pkg, _, _ := strings.Cut(name, "::")
		extension := sort.SearchStrings(packages, pkg) < len(packages) && packages[sort.SearchStrings(packages, pkg)] == pkg
		if !extension {
			continue
		}
		entry, ok := byName[name]
		if !ok {
			t.Errorf("OpenSysML binds %s in the runtime registry but engine-contract.json does not list it; %s", name, update)
			continue
		}
		if entry.Kind != enginecontract.KindFunction {
			t.Errorf("%s is a registry function but the manifest kind is %q; %s", name, entry.Kind, update)
			continue
		}
		registryParams := make([]string, len(fn.params))
		for i, p := range fn.params {
			registryParams[i] = p.name
		}
		if !equalParamNames(entry.Parameters, registryParams) {
			t.Errorf("%s parameters: registry %v != manifest %v; %s", name, registryParams, entry.Parameters, update)
		}
	}

	for _, entry := range manifest.Entries {
		if entry.Kind != enginecontract.KindFunction {
			continue
		}
		pkg, _, _ := strings.Cut(entry.Name, "::")
		if !registryPackages[pkg] {
			continue
		}
		if _, ok := libraryFunctions[entry.Name]; !ok {
			t.Errorf("engine-contract.json lists function %s but the runtime registry does not implement it; %s", entry.Name, update)
		}
	}
}

func equalParamNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
