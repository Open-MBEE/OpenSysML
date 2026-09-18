package resolve_test

import "testing"

// f68Clean resolves src and requires it to produce no diagnostics.
func f68Clean(t *testing.T, name, src string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		r, _, _ := resolvedDoc(t, src)
		if len(r.Diagnostics) != 0 {
			for _, d := range r.Diagnostics {
				t.Logf("diag: %s", d.Message)
			}
			t.Error("unexpected diagnostics above")
		}
	})
}

// f68Rejects resolves src and requires at least one diagnostic.
func f68Rejects(t *testing.T, name, src string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		r, _, _ := resolvedDoc(t, src)
		if len(r.Diagnostics) == 0 {
			t.Error("expected a diagnostic")
		}
	})
}

// A transition's trigger declares a payload parameter of the transition itself
// (SysML.xtext TransitionUsage: EmptyParameterMember TriggerActionMember), so a
// sibling reaches it through the transition's name (F68: `subscribing.sub`).
func TestF68TriggerPayloadThroughTransitionName(t *testing.T) {
	f68Clean(t, "guard-reads-payload", `package A1 {
		item def Subscribe { attribute topic; }
		port def SubscriptionPort;
		part server {
			port subscriptionPort : SubscriptionPort;
			exhibit state sb {
				entry; then w1;
				state w1;
				transition subscribing
					first w1
					accept sub : Subscribe via subscriptionPort
					then w2;
				state w2;
				transition delivering
					first w2
					if subscribing.sub.topic == "t"
					then w1;
			}
		}
	}`)
}

// A transition's own guard still reads the payload by plain name, unchanged.
func TestF68TriggerPayloadInOwnGuard(t *testing.T) {
	f68Clean(t, "own-guard", `package A2 {
		item def Subscribe { attribute topic; }
		part server {
			exhibit state sb {
				entry; then w1;
				state w1;
				transition first w1
					accept sub : Subscribe
					if sub.topic == "t"
					then w1;
			}
		}
	}`)
}

// The payload is a member of its own transition only: a name no trigger binds
// stays unresolved, and one trigger's payload is not another's.
func TestF68TriggerPayloadNegative(t *testing.T) {
	f68Rejects(t, "no-such-member", `package A3 {
		item def Subscribe;
		part server {
			exhibit state sb {
				entry; then w1;
				state w1;
				transition subscribing
					first w1
					accept sub : Subscribe
					then w1;
				ref bad = subscribing.nosuchthing;
			}
		}
	}`)
	f68Rejects(t, "other-transitions-payload", `package A4 {
		item def Subscribe;
		part server {
			exhibit state sb {
				entry; then w1;
				state w1;
				transition subscribing
					first w1
					accept sub : Subscribe
					then w2;
				state w2;
				transition delivering
					first w2
					then w1;
				ref bad = delivering.sub;
			}
		}
	}`)
}

// A feature that takes its name from a feature it redefines is a valid target
// for a reference subsetting: it is the feature of that name here, so its
// members resolve (F68: `producer.publish_request` under `part :>> producer`).
func TestF68RedefinitionNamedMemberIsReferenceTarget(t *testing.T) {
	f68Clean(t, "redefinition-named-member", `package M1 {
		part def PD { part producer; }
		part src { attribute a2; }
		part r : PD {
			part :>> producer :> src;
			event producer.a2[1];
		}
	}`)
	f68Clean(t, "flow-end-through-redefinition", `package M2 {
		port def PP;
		part def PD { part producer[1]; }
		part producer_2 { port publicationPort : PP; }
		part r : PD {
			part :>> producer :> producer_2;
			flow f {
				end ::> producer.publicationPort;
			}
		}
	}`)
}

// A feature that only borrowed its name from a reference subsetting is still no
// target for another one: `event host.inner[1]` binds `inner` without declaring
// it, so a sibling reference to `inner` must fail as the reference does.
func TestF68BorrowedNameIsNotAReferenceTarget(t *testing.T) {
	f68Rejects(t, "borrowed-name-hidden", `package N2 {
		part def PD { part inner; }
		part host : PD;
		part x {
			event host.inner[1];
			event inner[1];
		}
	}`)
}

// A transition with no trigger, and a chain through one, must degrade to
// diagnostics rather than panic.
func TestF68TriggerlessTransitionNoPanic(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package A5 {
		part server {
			exhibit state sb {
				entry; then w1;
				state w1;
				transition plain first w1 then w1;
				ref bad = plain.nope.deeper;
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for a member of a triggerless transition")
	}
}
