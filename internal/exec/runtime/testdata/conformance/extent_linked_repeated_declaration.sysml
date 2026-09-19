// `all D` reaches the objects held under every Node a run linked through `next`, however many
// Nodes the path repeats: only a feature that would create another object of a declaration on
// the path goes unread, so each Node's wheel is walked, depth first in feature declaration order.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel;
    part def Node {
        ref part next : Node[0..1];
        part wheel : Wheel;
    }
    part def Rig {
        part head : Node;
        part pool : Node[2];
    }
    part rig : Rig;

    action link {
        attribute wheeled : Boolean[*] nonunique;

        ref part n1 : Node = rig.pool#(1);
        ref part n2 : Node = rig.pool#(2);

        first start;
        action wire {
            assign rig.head.next := n1;
            assign n1.next := n2;
        }
        action check {
            assign wheeled := (
                size(all Node) == 3,
                size(all Wheel) == 3,
                (all Wheel)#(1) === n2.wheel,
                (all Wheel)#(2) === n1.wheel,
                (all Wheel)#(3) === rig.head.wheel
            );
        }
        done;

        succession first start then wire;
        succession first wire then check;
        succession first check then done;
    }
}
