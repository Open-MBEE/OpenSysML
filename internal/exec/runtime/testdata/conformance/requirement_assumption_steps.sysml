// An assumption whose body states steps is judged as one without: the condition
// its steps leave reading false makes the requirement hold whatever its required
// constraint reads (`allTrue(assumptions()) implies allTrue(constraints())`).
package test {
    private import ScalarValues::*;
    requirement guarded {
        attribute a : Real = 2.0;
        assume constraint { attribute b : Real = 1; assign b := b + 5; b <= 3 }
        require constraint { a > 10.0 }
    }
}
