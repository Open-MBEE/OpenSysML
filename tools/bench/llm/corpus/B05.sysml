package Payment {
    private import ScalarValues::*;

    action def Checkout {
        in amount : Real;
        out approved : Boolean;

        action checkBalance {
            in amount : Real;
            out sufficient : Boolean;
        }
        bind amount = checkBalance.amount;
        then decide;
            if checkBalance.sufficient then charge;
            else reject;
        action charge {
            out approved : Boolean;
        }
        action reject {
            out approved : Boolean;
        }
        then merge continue;
        then done;
    }
}
