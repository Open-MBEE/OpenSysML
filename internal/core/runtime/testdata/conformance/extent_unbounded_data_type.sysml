// `all T` over a data type declaring no values has no finite extent to enumerate: it
// is a typed refusal, not an empty sequence.
package test {
    private import ScalarValues::*;

    part counter {
        attribute integers = all Integer;
        attribute strings = all String;
    }
}
