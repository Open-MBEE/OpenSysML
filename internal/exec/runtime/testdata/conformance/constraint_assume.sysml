package test {
    // Assume is treated as non-failing precondition
    constraint TestAssume {
        assume constraint { 1 > 100 }  // false assumption, but constraint still satisfied
    }
}
