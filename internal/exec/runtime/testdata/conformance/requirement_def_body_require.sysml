// A requirement definition states a reusable condition over its own attributes,
// declared before the `require` member.
package test {
    requirement def MassBudgetCheck {
        attribute actual = 1.2;
        attribute limit = 1.5;
        require constraint { actual <= limit }
    }
}
