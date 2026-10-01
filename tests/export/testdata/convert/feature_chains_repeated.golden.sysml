package P {
    part def Node {
        part next : Node;
    }
    part n : Node;
    part def H {
        part q : Node;
    }
    part h : H {
        ref r = n.next.next;
    }
    connection c connect n.next.next to h.q;
}
