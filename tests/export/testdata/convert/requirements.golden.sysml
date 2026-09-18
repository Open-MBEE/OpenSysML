package Reqs {
    requirement def Safe {
        attribute margin : Real;
    }
    constraint def Positive {
        attribute value : Real;
    }
    part def System {
        requirement safety : Safe;
    }
}
