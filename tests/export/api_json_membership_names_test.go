package export_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// KerML 10.4.5: an owning membership states memberName and memberShortName beside the
// differently named ownedMemberName and ownedMemberShortName that redefine them, with their values.
func TestAPIJSONOwningMembershipStatesRedefinedMemberNames(t *testing.T) {
	t.Parallel()
	redefined := map[string]string{"memberName": "ownedMemberName", "memberShortName": "ownedMemberShortName"}
	for _, path := range modelFiles(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			document, err := cachedAPIJSON(path, source, "standard", "graph")
			if err != nil {
				t.Fatal(err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatal(err)
			}
			for _, element := range elements {
				var metaclass string
				if err := json.Unmarshal(element["@type"], &metaclass); err != nil {
					t.Fatal(err)
				}
				if !ontology.IsAncestorOrSelf(apiJSONTestOntologyMetaclass(metaclass), "OwningMembership") {
					continue
				}
				for property, redefining := range redefined {
					value, stated := element[property]
					if !stated {
						t.Errorf("%s %s states no %s", metaclass, element["@id"], property)
						continue
					}
					owned, completed := element[redefining]
					if !completed {
						owned = []byte("null")
					}
					if !bytes.Equal(bytes.TrimSpace(value), bytes.TrimSpace(owned)) {
						t.Errorf("%s %s: %s = %s, %s = %s", metaclass, element["@id"], property, value, redefining, owned)
					}
				}
			}
		})
	}
}
