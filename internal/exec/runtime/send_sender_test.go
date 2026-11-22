package runtime

import "testing"

const sendSenderModel = `package Test {
	item def Ping;
	item def Go;
	state def Looped {
		entry; then waiting;
		state waiting {
			item kept : Ping[*] ordered = (new Ping(), new Ping());
			exit action flush {
				for k in kept { send k to self; }
			}
		}
		state open;
		transition first waiting accept Go then open;
	}
	state def Plain {
		entry; then waiting;
		state waiting {
			exit action flush { send new Ping() to self; }
		}
		state open;
		transition first waiting accept Go then open;
	}
	part def Owner {
		state looped : Looped;
		state plain : Plain;
	}
	part owner : Owner;
}`

// A send that states no sender sends from `this` (SysML v2 §8.4.13.5; `SendPerformance::sender
// default this`): the run's own performance, or the part a composite behavior belongs to.
func TestSendDefaultSenderIsThis(t *testing.T) {
	for _, tc := range []struct {
		name, machine, owner string
		transfers            int
	}{
		{"a definition run sending from a loop body", "Test::Looped", "", 2},
		{"a definition run sending outside any loop", "Test::Plain", "", 1},
		{"a part's state sending from a loop body", "Test::Owner::looped", "Test::owner", 2},
		{"a part's state sending outside any loop", "Test::Owner::plain", "Test::owner", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, idx := libraryModelContext(t, sendSenderModel)
			var self *Instance
			if tc.owner != "" {
				var err error
				if self, err = ctx.Instantiate(lookupOne(t, idx, tc.owner)); err != nil {
					t.Fatalf("instantiate %s: %v", tc.owner, err)
				}
			}
			exec, err := ctx.CreateStateExecutorFor(lookupOne(t, idx, tc.machine), self)
			if err != nil {
				t.Fatalf("create state executor: %v", err)
			}
			t.Cleanup(exec.Release)
			if err := exec.RunToQuiescence(); err != nil {
				t.Fatalf("start: %v", err)
			}
			exec.SendSignal("Go", nil)
			if err := exec.RunToQuiescence(); err != nil {
				t.Fatalf("run: %v", err)
			}
			sender := self
			if sender == nil {
				sender = exec.occurrence
			}
			if sender == nil {
				t.Fatal("the run has no occurrence to send from")
			}
			if len(ctx.transfers) != tc.transfers {
				t.Fatalf("%d transfers recorded, want %d", len(ctx.transfers), tc.transfers)
			}
			for _, record := range ctx.transfers {
				if record.source != sender.ID {
					t.Errorf("transfer #%d is from object #%d (%s), want #%d (%s)", record.id,
						record.source, ctx.instances[record.source].Type.Name, sender.ID, sender.Type.Name)
				}
			}
		})
	}
}
