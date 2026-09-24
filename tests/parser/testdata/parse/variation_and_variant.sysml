// Variation points and their variants, in attribute, part and interface form.
package variability {
    part def Engine;

    part vehicleFamily {
        variation attribute cut {
            variant attribute cutShallow { :>> cost = 200.0; }
            variant attribute cutIdeal { :>> cost = 250.0; }
        }

        variation part engine : Engine {
            variant part electric : Engine { attribute :>> mass = 300.0; }
            variant part petrol : Engine;
        }

        variation interface engagementRingToBand {
            variant interface connected
                connect engagementRing.ringPort to band.ringPort;

            variant interface disconnected
                connect diamond.bandPort to band.diamondPort;
        }
    }

    part chosenVehicle :> vehicleFamily {
        attribute :>> cut = cut::cutIdeal;
        part :>> engine = engine::electric;
        ref :>> engagementRingToBand = engagementRingToBand::connected;

        assert constraint isIdeal {
            cut == cut::cutIdeal
        }
    }
}
