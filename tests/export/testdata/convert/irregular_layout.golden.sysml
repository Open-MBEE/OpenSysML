package IrregularLayout {
    private import ScalarValues::*;
    doc /* A model whose author laid the heads out
       *   over several lines,
	 *	with tabs,

       * blank lines and odd indentation. */
    calc def Speed { in m : Real; return : Real; }
    part def Wheel;
    part def Axle;
    part def Rover {
        doc /* the rover:
             its doc spans
                lines too */
        attribute mass : Real = 80;
        attribute limit : Real =
                100
              + 20;
        attribute label =
            "two\nlines";
        attribute	tabbed	:	Real	=	1;
        part
            wheels
                : Wheel[4];
        part axle :
            Axle;
        connect wheels
            to axle;
        constraint bounds {
            mass > 0
                and
              mass < limit
        }
        assert constraint {
                mass
                    >
                0
        }
        calc speed : Speed {
            in m =
                mass * 2
                    - 1;
        }
        comment about Rover /* a comment
   that wanders back
			  and forth */
    }
    action def Drive {
        in attribute count;
        attribute total;
        action tick;
        if count > 0
              and total > 0 {
            perform tick;
        }
        while count
                > 0 {
            perform tick;
        }
    }
    state def Ops {
        entry; then off;
        state off;
        state on;
        transition first off
            if 1 > 0
            then on;
    }
}
