package opensysml

import (
	"context"
	"testing"
)

// A call or option the service names a capability for is refused before the request leaves the client
// when the service does not announce it: oldCaller answers no call, so one that was sent fails the test.
func TestCallsAreNotSentWithoutTheirCapability(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	inline := []Document{Source("a.sysml", "package A;")}
	for _, tc := range []struct {
		name       string
		capability string
		// held are the capabilities the service does announce: the call's own, when an option is what is refused.
		held []string
		call func(c *client) error
	}{
		{"Convert", CapabilityConvert, nil, func(c *client) error {
			_, err := c.Convert(ctx, model, FormatSysML)
			return err
		}},
		{"MigrateSource", CapabilityMigrate, nil, func(c *client) error {
			_, err := c.MigrateSource(ctx, []byte("<xmi/>"), FormatSysML)
			return err
		}},
		{"ApplyEdits", CapabilityApplyEdits, []string{CapabilityEditDocuments}, func(c *client) error {
			_, err := c.ApplyEdits(ctx, model, Rename{Target: "A", NewName: "B"})
			return err
		}},
		{"ParseDocuments", CapabilityParseSources, nil, func(c *client) error {
			_, err := c.ParseDocuments(ctx, inline)
			return err
		}},
		{"ParseDocuments WithStrictConformance", CapabilityStrictConformance, []string{CapabilityParseSources}, func(c *client) error {
			_, err := c.ParseDocuments(ctx, inline, WithStrictConformance())
			return err
		}},
		{"ParseDocuments WithLanguage", CapabilityInlineLanguage, []string{CapabilityParseSources}, func(c *client) error {
			_, err := c.ParseDocuments(ctx, inline, WithLanguage(LanguageSysML))
			return err
		}},
		{"ParseSource WithStrictConformance", CapabilityStrictConformance, nil, func(c *client) error {
			_, err := c.ParseSource(ctx, "package A;", WithStrictConformance())
			return err
		}},
		{"ParseSource WithLanguage", CapabilityInlineLanguage, nil, func(c *client) error {
			_, err := c.ParseSource(ctx, "package A;", WithLanguage(LanguageSysML))
			return err
		}},
		{"Evaluate WithSubject", CapabilityEvaluateSubject, nil, func(c *client) error {
			_, err := c.Evaluate(ctx, model, "1 + 1", WithSubject("A"))
			return err
		}},
		{"Query", CapabilityQuery, nil, func(c *client) error {
			_, err := c.Query(ctx, model, Query{})
			return err
		}},
		{"Query ordered", CapabilityQueryOrderBy, []string{CapabilityQuery}, func(c *client) error {
			_, err := c.Query(ctx, model, Query{OrderBy: []string{"name"}})
			return err
		}},
		{"QueryOSLC", CapabilityOSLCQuery, []string{CapabilityQuery}, func(c *client) error {
			_, err := c.QueryOSLC(ctx, model, `oslc.where=name="A"`)
			return err
		}},
		{"ElementProperties", CapabilityElementProperties, nil, func(c *client) error {
			_, err := c.ElementProperties(ctx, model, "A")
			return err
		}},
		{"RunDocumentQuery", CapabilityDocumentQuery, nil, func(c *client) error {
			_, err := c.RunDocumentQuery(ctx, model, "A::q")
			return err
		}},
		{"RenderDocument", CapabilityRenderDocument, nil, func(c *client) error {
			_, err := c.RenderDocument(ctx, model, "A::d")
			return err
		}},
		{"VerifyConstraint", CapabilityVerification, nil, func(c *client) error {
			_, err := c.VerifyConstraint(ctx, model, "A")
			return err
		}},
		{"VerifyRequirement", CapabilityVerification, nil, func(c *client) error {
			_, err := c.VerifyRequirement(ctx, model, "A")
			return err
		}},
		{"VerifySatisfaction", CapabilityVerification, nil, func(c *client) error {
			_, err := c.VerifySatisfaction(ctx, model, "A")
			return err
		}},
		{"ValidateInstance", CapabilityVerification, nil, func(c *client) error {
			_, err := c.ValidateInstance(ctx, model, "A")
			return err
		}},
		{"EvaluateCalc", CapabilityVerification, nil, func(c *client) error {
			_, err := c.EvaluateCalc(ctx, model, "A")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &client{caller: &oldCaller{t: t, capabilities: tc.held}}
			wantMissingCapability(t, tc.name, tc.call(c), tc.capability)
		})
	}
}
