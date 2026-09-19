package test {
    // A transition endpoint is a resolved name, not a name matched among the
    // machine's states: `beta::work` names the substate of `beta` even though
    // `alpha` declares a state of the same name, and `marker` ends at 2 only if
    // the transition entered the state the qualified name resolves to.
    state TestTransitionEndpointQualified {
        attribute marker : Integer = 0;

        entry; then start;
        state start;
        state alpha {
            state work {
                entry { assign marker := 1; }
            }
        }
        state beta {
            state work {
                entry { assign marker := 2; }
            }
        }

        transition first start then beta::work;
        transition first beta::work when Leave then done;
    }
}
