package q {
    private import ISQBase::DurationValue;
    private import SI::s;
    private import SIPrefixes::milli;
    part def T { attribute period : DurationValue; }
    part t : T { :>> period = 20 [milli*s]; }
}
