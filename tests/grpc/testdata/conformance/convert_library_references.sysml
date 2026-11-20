package q {
    private import ISQ::*;
    private import SI::*;
    private import SIPrefixes::*;
    part def T { attribute period : DurationValue; }
    part t : T { :>> period = 20 [milli*s]; }
}
