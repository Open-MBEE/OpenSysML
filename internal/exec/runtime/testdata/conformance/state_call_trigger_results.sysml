package CallTriggerResults {
    // A caller of an operation the machine handles as a call event is released
    // once the run-to-completion step dispatching it ends, with the outputs the
    // behaviors that step triggered returned to the machine: the first call's
    // effect writes `result` and the entry of the state it enters negates it, so
    // the entry's value is the one that call returns; the second call's effect
    // alone writes it.
    state Calculator {
        action def Double {
            in value : Integer;
            out result : Integer;

            first start;
            action doubling {
                assign result := value * 2;
            }
            done;
            succession first start then doubling;
            succession first doubling then done;
        }

        action def Negate {
            inout result : Integer;

            first start;
            action negating {
                assign result := 0 - result;
            }
            done;
            succession first start then negating;
            succession first negating then done;
        }

        attribute result : Integer = 0;

        entry; then idle;
        state idle;
        state doubled {
            entry perform Negate;
        }
        state settled;

        transition first idle accept compute(value) do action doubling : Double { in value = value; } then doubled;
        transition first doubled accept compute(value) do action doubling : Double { in value = value; } then settled;
        transition first settled accept Settle then done;
    }
}
