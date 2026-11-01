package Lander {
    private import ScalarValues::*;

    port def Harness;

    part def Payload {
        attribute mass : Real[1];
        port link : Harness;
    }

    requirement def PositivePayloadMass {
        subject payload : Payload;
        require constraint { payload.mass > 0.0 }
    }
    requirement positivePayload : PositivePayloadMass;

    part lander {
        part cabin : Payload {
            attribute redefines mass = 320.0;
        }
        part engine : Payload {
            attribute redefines mass = 145.0;
        }
        part scienceBay : Payload;
        connection cabinEngine connect cabin.link to engine.link;
        connection engineBay connect engine.link to scienceBay.link;
        assert satisfy positivePayload by cabin;
    }

    package Empty {
    }
}
