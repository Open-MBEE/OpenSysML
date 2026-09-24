package test {
    // A negation denies the conditions of its constraint together, `not (a and b)`
    // rather than `not a and not b`, so it holds as soon as one of them fails.
    constraint NegatedGroup {
        assert not constraint {
            1 < 2
            2 < 1
        }
    }
}
