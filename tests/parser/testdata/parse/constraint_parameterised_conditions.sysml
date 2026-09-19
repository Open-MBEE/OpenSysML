// A constraint body that declares parameters is read by parseCalcBody, so a bare
// expression there is a condition rather than a calculated result, including one
// that starts with a keyword; `binding … of …` records a reference.
part def Tank {
    attribute level : Real;
    attribute full : Boolean;

    constraint notOverfull {
        in x : Real;
        x >= 0
        not (x > 100)
    }

    constraint keywordInitial {
        in x : Real;
        true
        not (false)
        null != x
        if x > 0 ? true else false
    }

    binding levelIsFull of full = level;
}
