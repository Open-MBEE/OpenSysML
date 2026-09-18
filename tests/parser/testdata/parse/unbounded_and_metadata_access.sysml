package test {
    metadata def Safety {
        attribute level;
    }

    part def Vehicle;

    part seatBelt : Vehicle {
        @Safety {
            level = 4;
        }
    }

    part def Report {
        attribute unbounded = *;
        attribute exceedsFinite = * > 1000;
        attribute annotations = seatBelt.metadata;
        attribute qualified = test::seatBelt.metadata;
        attribute level = seatBelt.metadata#(1).level;
    }
}
