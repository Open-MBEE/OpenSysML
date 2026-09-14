// An assertion that cannot be evaluated is undecided, not violated: the tank's
// capacity has no value, so the constraint reading it and the requirement bound
// to it decide nothing, while the assertion reading a valued feature holds. An
// undecided verdict leaves the object not valid, since nothing was shown.
package test {
    private import ScalarValues::Real;

    part def Tank {
        attribute capacity : Real;
        attribute level : Real = 10.0;
        assert constraint levelPositive { level > 0.0 }
        assert constraint fits { level <= capacity }
        requirement roomLeft {
            attribute c : Real = capacity;
            require constraint { c > 0.0 }
        }
    }

    part tank : Tank;
}
