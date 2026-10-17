// The definitions of `part_owned_action_writes_owner_across_documents`, indexed beside it:
// the action usage `fill` is a feature of `Plant`, so the names its body writes, `runs` and
// `tank.volume`, resolve to features of the part performing it.
package plantDefs {
    private import ScalarValues::*;

    part def Tank {
        attribute volume : Real default = 1.0;
    }

    part def Plant {
        part tank : Tank;
        attribute runs : Integer default = 0;

        action fill {
            action 'fill tank' {
                assign tank.volume := tank.volume * 2 + 2;
            }
            first start then 'fill tank';
            action 'count run' {
                assign runs := runs + 1;
            }
            first 'fill tank' then 'count run';
        }
    }
}
