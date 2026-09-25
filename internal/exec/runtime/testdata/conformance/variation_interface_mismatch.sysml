// The constraint relating a variant choice to a connection choice is violated
// when the two do not agree, and reports a verdict rather than an error.
package test {
    private import ScalarValues::Real;

    port def RingPort;
    port def DiamondPort;

    attribute def Nesting {
        attribute cost : Real;
    }

    part def Ring {
        attribute nesting : Nesting;
        port ringPort : RingPort;
        port diamondPort : DiamondPort;
    }

    part def Assembly {
        part engagementRing : Ring;
        part band : Ring;
    }

    abstract part assemblyFamily : Assembly {
        part :>> engagementRing {
            variation attribute :>> nesting {
                variant attribute nestingTrue { :>> cost = 100.0; }
                variant attribute nestingFalse { :>> cost = 200.0; }
            }
        }

        variation interface engagementRingToBand {
            variant interface connected
                connect engagementRing.ringPort to band.ringPort;

            variant interface disconnected
                connect engagementRing.diamondPort to band.diamondPort;
        }

        assert constraint nestingMatchesConnection {
            engagementRing.nesting == engagementRing.nesting::nestingTrue and
                engagementRingToBand == engagementRingToBand::connected xor
            engagementRing.nesting == engagementRing.nesting::nestingFalse and
                engagementRingToBand == engagementRingToBand::disconnected
        }
    }

    part inconsistentAssembly :> assemblyFamily {
        part :>> engagementRing {
            attribute :>> nesting = nesting::nestingFalse;
        }
        ref :>> engagementRingToBand = engagementRingToBand::connected;
    }
}
