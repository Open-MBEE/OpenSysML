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
        transition idle accept setSpeed(value) if value > 0 then fast;
        transition fast accept w : Speed if w > 0 then idle;
    }
}
