package Sets {
    action def Measure {
        in a : ScalarValues::Real[1];
        in b : ScalarValues::Real[0..*];
        out r : ScalarValues::Real[1];
        ref inputs {
            ref [1] = a;
        }
        ref results {
            ref [0..*] = b;
            ref [1] = r;
        }
    }
    part def Tool {
        abstract action run {
            in x : ScalarValues::Real[1];
            ref args {
                ref [1] = x;
            }
        }
        action def Spin {
            in v : ScalarValues::Real[1];
            ref {
                ref [1] = v;
            }
            /* not migrated: Constraint 'positive' — a parameter set's condition has no target in the transformation */
            ref bounded {
                ref [1] = v;
            }
        }
    }
}
