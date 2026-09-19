// `all D` walks the objects a run linked through a recursive composition: a Node the walk
// reached through `next` has its own `next` read, so the links order the extent, not the
// pool the nodes were declared in.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Node { part next : Node[0..1]; }
    part def Rig {
        part head : Node;
        part pool : Node[4];
    }
    part rig : Rig;

    action link {
        attribute chained : Boolean[*] nonunique;

        ref part n1 : Node = rig.pool#(1);
        ref part n2 : Node = rig.pool#(2);
        ref part n3 : Node = rig.pool#(3);
        ref part n4 : Node = rig.pool#(4);

        first start;
        action wire {
            assign rig.head.next := n1;
            assign n1.next := n3;
            assign n3.next := n4;
            assign n4.next := n2;
        }
        action check {
            assign chained := (
                size(all Node) == 5,
                (all Node)#(1) === rig.head,
                (all Node)#(2) === rig.pool#(1),
                (all Node)#(3) === rig.pool#(3),
                (all Node)#(4) === rig.pool#(4),
                (all Node)#(5) === rig.pool#(2)
            );
        }
        done;

        succession first start then wire;
        succession first wire then check;
        succession first check then done;
    }
}
