// A feature valued by a chain over a collection holds the chain's values from
// every member; a collection short of its lower bound is made up through its
// optional subsetting features before anonymous members.
package test {
    item def Corner;

    item def Side {
        item corners : Corner [2];
    }

    item def Panel {
        item sides : Side [3];
        item left : Side [1] :> sides;
        item right : Side [1] :> sides;
        item top : Side [0..1] :> sides;
        item corners : Corner [*] = sides.corners;
    }

    part def Rack {
        item panel : Panel;
    }
}
