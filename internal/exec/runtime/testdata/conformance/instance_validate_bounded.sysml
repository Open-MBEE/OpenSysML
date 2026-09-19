// A type holding a part of its own type nests without end, so a validation
// stops at the part that would begin a type already being expanded, reporting
// the walk as bounded: the unreached nesting is unvalidated rather than valid.
// The assertions of the objects reached before the cut are still checked.
package test {
    private import ScalarValues::Integer;

    part def Node {
        attribute depth : Integer = 1;
        part next : Node;
        assert constraint shallow { depth < 3 }
    }

    part node : Node;
}
