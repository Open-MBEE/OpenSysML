// Derived from Open-MBEE's BallAndChain: the untyped `interface
// engagementRingToDiamond` usage and the `variation interface
// engagementRingToBand` whose variants connect different pairs of ports. Here
// the disconnected variant is selected, so its connection is the realized one
// and the connected variant's is not.
package test {
    port def EngagementRingPort;
    port def BandPort;
    port def RingPort;
    port def DiamondPort;

    part def Diamond {
        port engagementRingPort : EngagementRingPort;
        port bandPort : BandPort;
    }

    part def Ring {
        port ringPort : RingPort;
        port diamondPort : DiamondPort;
    }

    part def Band :> Ring;
    part def EngagementRing :> Ring;

    abstract part ballAndChainFamily {
        part diamond : Diamond;
        part band : Band;
        part engagementRing : EngagementRing;

        interface engagementRingToDiamond
            connect engagementRing.diamondPort to diamond.engagementRingPort;

        variation interface engagementRingToBand {
            variant interface engagementRingToBandConnected
                connect engagementRing.ringPort to band.ringPort;

            variant interface engagementRingToBandDisconnected
                connect diamond.bandPort to band.diamondPort;
        }
    }

    // `ref` because an interface usage carries no feature value.
    part newBallAndChain :> ballAndChainFamily {
        ref :>> engagementRingToBand = engagementRingToBand::engagementRingToBandDisconnected;
    }
}
