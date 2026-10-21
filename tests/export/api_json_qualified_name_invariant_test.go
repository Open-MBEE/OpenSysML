package export_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
)

func TestAPIJSONDeclaredQualifiedNamesMatchOwningNamespace(t *testing.T) {
	paths, exampleRoot := apiJSONQualifiedNamePaths(t)
	var countsMu sync.Mutex
	fixtures, examples, skippedExamples, kindQualifiedNames := 0, 0, 0, 0
	var offenders []string
	t.Cleanup(func() {
		countsMu.Lock()
		defer countsMu.Unlock()
		sort.Strings(offenders)
		t.Logf("qualified-name invariant inputs: %d convert fixtures, %d converting examples, %d skipped examples; %d qualifiedNames end in ::kind",
			fixtures, examples, skippedExamples, kindQualifiedNames)
		if len(offenders) > 0 {
			const limit = 20
			if len(offenders) > limit {
				t.Errorf("%d declared qualifiedNames differ from their owning namespace; first %d:\n%s",
					len(offenders), limit, strings.Join(offenders[:limit], "\n"))
				return
			}
			t.Errorf("%d declared qualifiedNames differ from their owning namespace:\n%s",
				len(offenders), strings.Join(offenders, "\n"))
		}
	})

	for _, path := range paths {
		path := path
		isExample := strings.HasPrefix(path, exampleRoot+string(filepath.Separator))
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := cachedSysMLToRDF(path, source); err != nil {
				if isExample {
					countsMu.Lock()
					skippedExamples++
					countsMu.Unlock()
					t.Logf("skipping non-converting example: %v", err)
					return
				}
				t.Fatalf("convert fixture: %v", err)
			}
			document, err := cachedAPIJSON(path, source, "standard", "graph")
			if err != nil {
				t.Fatalf("WriteAPIJSON: %v", err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatalf("decode API JSON: %v", err)
			}
			byID := make(map[string]map[string]json.RawMessage, len(elements))
			for _, element := range elements {
				if id, ok := apiJSONString(element["@id"]); ok {
					byID[id] = element
				}
			}

			var localOffenders []string
			localKindQualifiedNames := 0
			for _, element := range elements {
				declaredName, hasDeclaredName := apiJSONString(element["declaredName"])
				qualifiedName, hasQualifiedName := apiJSONString(element["qualifiedName"])
				if hasQualifiedName && strings.HasSuffix(qualifiedName, "::kind") {
					localKindQualifiedNames++
				}
				if !hasDeclaredName || !hasQualifiedName {
					continue
				}
				expected, reason := apiJSONExpectedQualifiedName(element, byID, declaredName)
				id, _ := apiJSONString(element["@id"])
				if reason != "" {
					localOffenders = append(localOffenders, fmt.Sprintf(
						"%s @id=%q declaredName=%q qualifiedName=%q: %s",
						filepath.ToSlash(path), id, declaredName, qualifiedName, reason,
					))
					continue
				}
				if qualifiedName != expected {
					localOffenders = append(localOffenders, fmt.Sprintf(
						"%s @id=%q declaredName=%q qualifiedName=%q want=%q",
						filepath.ToSlash(path), id, declaredName, qualifiedName, expected,
					))
				}
			}

			countsMu.Lock()
			defer countsMu.Unlock()
			if isExample {
				examples++
			} else {
				fixtures++
			}
			kindQualifiedNames += localKindQualifiedNames
			offenders = append(offenders, localOffenders...)
		})
	}
}

func apiJSONExpectedQualifiedName(
	element map[string]json.RawMessage,
	byID map[string]map[string]json.RawMessage,
	declaredName string,
) (string, string) {
	escapedName := identity.EscapeName(declaredName)
	relationshipID := apiJSONReferenceID(element["owningRelationship"])
	if relationshipID == "" {
		return escapedName, ""
	}
	relationship, ok := byID[relationshipID]
	if !ok {
		return "", fmt.Sprintf("owningRelationship @id %q is not in the output", relationshipID)
	}
	namespaceID := ""
	for _, property := range []string{"membershipOwningNamespace", "owner", "owningRelatedElement"} {
		if namespaceID = apiJSONReferenceID(relationship[property]); namespaceID != "" {
			break
		}
	}
	if namespaceID == "" {
		return escapedName, ""
	}
	namespace, ok := byID[namespaceID]
	if !ok {
		return "", fmt.Sprintf("owning namespace @id %q is not in the output", namespaceID)
	}
	namespaceName, hasNamespaceName := apiJSONString(namespace["declaredName"])
	namespaceQualifiedName, hasNamespaceQualifiedName := apiJSONString(namespace["qualifiedName"])
	if hasNamespaceQualifiedName {
		return namespaceQualifiedName + "::" + escapedName, ""
	}
	if hasNamespaceName && namespaceName != "" {
		return "", fmt.Sprintf("owning namespace @id %q has declaredName but no qualifiedName", namespaceID)
	}
	return escapedName, ""
}

func apiJSONString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func apiJSONReferenceID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var reference struct {
		ID string `json:"@id"`
	}
	if err := json.Unmarshal(raw, &reference); err == nil && reference.ID != "" {
		return reference.ID
	}
	var id string
	if err := json.Unmarshal(raw, &id); err == nil {
		return id
	}
	var references []json.RawMessage
	if err := json.Unmarshal(raw, &references); err == nil && len(references) > 0 {
		return apiJSONReferenceID(references[0])
	}
	return ""
}
