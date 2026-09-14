// A part of the type being expanded above it is cut only where reading it would
// begin objects without end. An optional one holds nothing of itself, so the walk
// reads it, finds no nesting to leave unreached, and shows the finite object valid.
package test {
    private import ScalarValues::Integer;

    part def Node {
        attribute depth : Integer = 1;
        part next : Node[0..1];
        part more : Node[0..*];
        assert constraint shallow { depth < 3 }
    }

    part def Chain {
        part head : Node;
    }

    part chain : Chain;
}
