package test {
    private import ScalarValues::*;
    private import SI::*;

    state def Machine parallel {
        attribute left : Integer = 0;
        attribute right : Integer = 0;
        attribute late : Integer = 0;

        state listener {
            entry; then waiting;
            state waiting {
                do action listen {
                    first start;
                    then action wait accept after 2 [s];
                    then action mark assign late := 1;
                    then done;
                }
            }
            state heard;
            succession first waiting then heard;
        }

        state worker {
            entry; then looping;
            state looping {
                do action poll {
                    first start;
                    then fork split;
                    succession first split then a;
                    succession first split then b;

                    action a {
                        first start;
                        then merge again;
                        then action wait accept after 1 [s];
                        then action write assign left := left + 1;
                        then again;
                    }

                    action b {
                        first start;
                        then merge again;
                        then action wait accept after 1 [s];
                        then action write assign right := right + 1;
                        then again;
                    }
                }
            }
            transition first looping accept after 1 [s] then finished;
            state finished;
        }
    }
}
