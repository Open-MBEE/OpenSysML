package opensysml_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const elementPropertiesSource = `package EP {
    port def CmdPort { out attribute cmd : ScalarValues::Integer; }
    part def Unit { port p : CmdPort; }
    part def Pair :> Unit {
        part u1 : Unit;
        part u2 : Unit;
        connect u1 to u2;
    }
}
`

func elementPropertiesClients(t *testing.T) map[string]opensysml.Client {
	t.Helper()
	local, err := opensysml.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { local.Close() })
	return map[string]opensysml.Client{"inprocess": local, "remote": dialClient(t, startService(t))}
}

func TestElementPropertiesReadsSpecPropertiesOverBothTransports(t *testing.T) {
	for name, client := range elementPropertiesClients(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			model, err := client.ParseSource(ctx, elementPropertiesSource)
			if err != nil {
				t.Fatal(err)
			}
			read, err := client.ElementProperties(ctx, model, "EP::Pair", "inheritedFeature", "isAbstract", "connectorEnd")
			if err != nil {
				t.Fatal(err)
			}
			if read.Element.QualifiedName != "EP::Pair" || read.Element.Metaclass != "PartDefinition" || read.Element.ID == "" {
				t.Errorf("element = %+v", read.Element)
			}
			inherited := read.Property("inheritedFeature")
			if inherited == nil || inherited.DefiningClass != "Type" || !inherited.Many || !inherited.Derived || inherited.Unsupported != "" {
				t.Fatalf("inheritedFeature = %+v", inherited)
			}
			var names []string
			for _, element := range inherited.Elements() {
				names = append(names, element.QualifiedName)
			}
			if len(names) < 2 || names[0] != "EP::Unit::p" {
				t.Errorf("inheritedFeature = %v, want EP::Unit::p first, then the library's", names)
			}
			abstract := read.Property("isAbstract")
			if abstract == nil || len(abstract.Values) != 1 || abstract.Values[0].Bool == nil || *abstract.Values[0].Bool || abstract.Values[0].Text() != "false" {
				t.Errorf("isAbstract = %+v", abstract)
			}
			if undeclared := read.Property("connectorEnd"); undeclared == nil || undeclared.Unsupported == "" || undeclared.DefiningClass != "" {
				t.Errorf("connectorEnd on a part def = %+v", undeclared)
			}

			everything, err := client.ElementProperties(ctx, model, "EP::Pair::@2")
			if err != nil {
				t.Fatal(err)
			}
			if everything.Element.Metaclass != "ConnectionUsage" || everything.Element.QualifiedName != "" {
				t.Fatalf("EP::Pair::@2 = %+v", everything.Element)
			}
			related := everything.Property("relatedFeature")
			if related == nil || len(related.Elements()) != 2 || related.Elements()[0].QualifiedName != "EP::Pair::u1" || related.Elements()[1].QualifiedName != "EP::Pair::u2" {
				t.Errorf("relatedFeature = %+v", related)
			}
			again, err := client.ElementProperties(ctx, model, everything.Element.ID, "sourceFeature")
			if err != nil {
				t.Fatal(err)
			}
			if source := again.Property("sourceFeature"); source == nil || len(source.Values) != 1 || source.Values[0].Text() != "EP::Pair::u1" {
				t.Errorf("sourceFeature by form id = %+v", source)
			}

			_, err = client.ElementProperties(ctx, model, "EP::Nowhere", "name")
			var failure *opensysml.FailureError
			if !errors.As(err, &failure) || failure.Op != "ElementProperties" {
				t.Errorf("a missing element: %v", err)
			}
		})
	}
}
