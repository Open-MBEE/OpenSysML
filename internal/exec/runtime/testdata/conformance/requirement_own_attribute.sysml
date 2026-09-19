// A requirement's condition sees the requirement's own attributes.
package test {
    requirement massBudget {
        attribute actual = 1.2;
        attribute limit = 1.5;
        require constraint { actual <= limit }
    }
}
