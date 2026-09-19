package Shadows {
    attribute def Speed;
    action def Sweep {
        in items : Speed[*];
        attribute value : Speed;
        for value in items {
            attribute cur = value;
        }
    }
    state def Governor {
        attribute value : Speed;
        attribute w : Speed;
        state idle;
        state fast;
        transition idle then fast accept setSpeed(value) if value > 0;
        transition fast then idle accept w : Speed if w > 0;
    }
}
