package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeRobustnessConnectorObjects(t *testing.T) {
	t.Run("binding_end_names_no_feature", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				part def Sys {
					attribute x : Integer = 1;
					binding bnd bind x = missing;
				}
			}`)
		before := len(ctx.instances)
		_, err := inst.GetFeatureValue(ctx, "bnd")
		if !errors.Is(err, ErrConnectorEnd) {
			t.Fatalf("GetFeatureValue(bnd) = %v, want ErrConnectorEnd", err)
		}
		var endErr *ConnectorEndError
		if !errors.As(err, &endErr) {
			t.Fatalf("error = %T, want *ConnectorEndError", err)
		}
		if endErr.Location == "" || !strings.Contains(endErr.Location, "<test>") {
			t.Errorf("connector end error location = %q, want <test> location", endErr.Location)
		}
		if len(ctx.instances) != before {
			t.Errorf("failed binding materialized %d object(s)", len(ctx.instances)-before)
		}
	})

	t.Run("flow_end_names_no_feature", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				port def P;
				part def Sys {
					port p : P;
					flow f from p to missing;
				}
			}`)
		before := len(ctx.instances)
		_, err := inst.GetFeatureValue(ctx, "f")
		if !errors.Is(err, ErrConnectorEnd) {
			t.Fatalf("GetFeatureValue(f) = %v, want ErrConnectorEnd", err)
		}
		var endErr *ConnectorEndError
		if !errors.As(err, &endErr) {
			t.Fatalf("error = %T, want *ConnectorEndError", err)
		}
		if endErr.Location == "" || !strings.Contains(endErr.Location, "<test>") {
			t.Errorf("connector end error location = %q, want <test> location", endErr.Location)
		}
		if len(ctx.instances) != before {
			t.Errorf("failed flow materialized %d object(s)", len(ctx.instances)-before)
		}
	})

	t.Run("binding_with_no_valued_end_as_object", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				part def Sys {
					attribute x : Integer;
					attribute y : Integer;
					binding bnd bind x = y;
				}
			}`)
		_, err := inst.GetFeatureValue(ctx, "bnd")
		if !errors.Is(err, ErrConnectorEnd) && !errors.Is(err, ErrBindingEnd) {
			t.Fatalf("GetFeatureValue(bnd) = %v, want connector or binding end error", err)
		}
	})

	t.Run("one_ended_binding_is_no_connector_object", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				part def Sys {
					attribute x : Integer = 1;
					binding b of x;
				}
			}`)
		owned, err := inst.OwnedConnectors(ctx)
		if err != nil {
			t.Fatalf("OwnedConnectors: %v", err)
		}
		if len(owned) != 0 {
			t.Fatalf("owned connectors = %d, want none", len(owned))
		}
		if _, err := inst.GetFeatureValue(ctx, "b"); err != nil && errors.Is(err, ErrConnectorEnd) {
			t.Fatalf("one-ended binding read as connector: %v", err)
		}
	})
}
