package runtime

import (
	"errors"
	"testing"
)

// A connection usage whose ends are declared in its body (`end :> causes ::> a;`)
// is a connector object like one with a `connect` clause: an end attaching to
// nothing is a typed error and makes no object, and a cycle through the ends is refused.
func TestRuntimeRobustnessBodyEndConnectors(t *testing.T) {
	t.Run("body_end_names_no_feature", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				connection def Pair { end one : Occurrence; end two : Occurrence; }
				part def Sys {
					occurrence a;
					connection pair : Pair {
						end :>> one ::> a;
						end :>> two ::> missing;
					}
				}
			}`)
		before := len(ctx.instances)
		_, err := inst.GetFeatureValue(ctx, "pair")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("GetFeatureValue(pair) = %v, want errConnectorEnd", err)
		}
		var endErr *connectorEndError
		if !errors.As(err, &endErr) {
			t.Fatalf("error = %T, want *connectorEndError", err)
		}
		if endErr.Location == "" {
			t.Errorf("connector end error carries no location")
		}
		if len(ctx.instances) != before {
			t.Errorf("failed connector materialized %d object(s)", len(ctx.instances)-before)
		}
	})

	t.Run("body_end_attaches_the_connector_itself", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				connection def Pair { end one : Occurrence; end two : Occurrence; }
				part def Sys {
					occurrence a;
					connection pair : Pair {
						end :>> one ::> a;
						end :>> two ::> pair;
					}
				}
			}`)
		_, err := inst.GetFeatureValue(ctx, "pair")
		if !errors.Is(err, ErrCyclicFeatureValue) && !errors.Is(err, errConnectorEnd) {
			t.Fatalf("GetFeatureValue(pair) = %v, want ErrCyclicFeatureValue or errConnectorEnd", err)
		}
	})

	t.Run("unattached_body_end_is_no_connector_object", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				connection def Pair { end one : Occurrence; end two : Occurrence; }
				part def Sys {
					occurrence a;
					connection pair : Pair {
						end :>> one ::> a;
						end :>> two;
					}
				}
			}`)
		fv, err := inst.GetFeatureValue(ctx, "pair")
		if err != nil {
			t.Fatalf("GetFeatureValue(pair) = %v, want an object of the usage", err)
		}
		if fv.HeldValue().Kind != ValInstance {
			t.Fatalf("pair holds %s, want an object", FormatValue(fv.HeldValue()))
		}
		if ctx.model.semantics.IsConnectorUsage(fv.Feature.Symbol) {
			t.Errorf("a connection with an unattached body end is reported as a connector usage")
		}
	})
}
