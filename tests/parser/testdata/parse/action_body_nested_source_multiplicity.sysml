package NestedSourceMultiplicity {
    action def A {
        action outer;
        if true {
            action ifSource;
            [1] then done;
            [0..1] then ifSource;
        } else {
            action elseSource;
            [1] then done;
            [0..1] then elseSource;
        }
        while true {
            action whileSource;
            [1] then done;
            [0..1] then whileSource;
        }
        loop {
            action loopSource;
            [1] then done;
            [0..1] then loopSource;
        }
        for i in (1, 2) {
            action forSource;
            [1] then done;
            [0..1] then forSource;
        }
    }
}
