package Waits {
    private import ISQ::DurationValue;
    action def Wait {
        in howLong : DurationValue;
        first start;
        then action w accept after howLong;
        then done;
    }
    action wait : Wait { in howLong = 20.0 [s]; }
}
