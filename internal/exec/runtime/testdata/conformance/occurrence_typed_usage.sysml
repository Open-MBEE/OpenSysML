package Sequences {
    part def Node {
        attribute ready : ScalarValues::Boolean = true;
    }
    item def Hello;
    item def Ack;
    item def Bye;

    occurrence def Handshake {
        ref part a : Node {
            event occurrence hello_sent;
            then event occurrence ack_received;
        }
        ref part b : Node {
            event occurrence hello_received;
            then event occurrence ack_sent;
        }
        message hello of Hello[1] from a.hello_sent to b.hello_received;
        message ack of Ack[1] from b.ack_sent to a.ack_received;
    }

    occurrence def Session {
        ref part a : Node {
            event occurrence opened;
            then event occurrence closed {
                assert constraint { ready }
            }
        }
        ref part b : Node {
            event occurrence bye_received;
        }
        occurrence shake : Handshake;
        succession first a.opened then shake;
        succession first shake then a.closed;
        message bye of Bye[1] from a.closed to b.bye_received;
    }

    part def Rig {
        part x : Node;
        part y : Node;
        occurrence session : Session {
            ref part :>> a = x;
            ref part :>> b = y;
        }
    }
}
