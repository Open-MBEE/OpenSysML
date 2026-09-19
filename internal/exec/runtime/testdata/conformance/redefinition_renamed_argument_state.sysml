// An object exhibits a state machine with an argument redefining a parameter
// under a name of its own, a short name alone included: the machine's entry
// action reads the redefined parameter's name and finds the argument's value.
package test {
    private import ScalarValues::*;

    state def Counting {
        in count : Integer default 1;
        attribute total : Integer = 0;

        entry; then active;
        state active {
            entry action increment {
                assign total := count + 1;
            }
        }
    }

    part def ConfiguredCounter {
        exhibit state short : Counting { in <c> :>> count = 5; }
        exhibit state named : Counting { in n :>> count = 8; }
    }
}
