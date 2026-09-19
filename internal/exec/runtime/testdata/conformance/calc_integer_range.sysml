package test {
    private import ScalarValues::*;
    private import ControlFunctions::*;

    // `..` is the ordered sequence of integers the library declares it to be, so
    // the sequence operations, indexing and `for` apply to it unchanged and a
    // descending range is simply empty.
    calc def RangeSum {
        in n : Integer;
        attribute squares = (1..n)->collect { in i; i * i };
        attribute picked = (1..n)->select { in i; i > 2 };
        attribute empty = n..1;
        attribute total : Integer = 0;
        for i in 1..n {
            assign total := total + i;
        }
        return : Integer =
            total
            + squares->NumericalFunctions::sum()
            + picked->SequenceFunctions::size()
            + empty->SequenceFunctions::size()
            + (1..n)#(2);
    }
}
