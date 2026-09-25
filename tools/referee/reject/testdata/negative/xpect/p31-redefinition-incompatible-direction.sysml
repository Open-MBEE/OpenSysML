// Invalid: A redefining feature must have a compatible direction (KerML 7.4.7 validateRedefinitionDirection; pilot validateRedefinitionDirectionConformance, PortUsage_Invalid.sysml.xt:57).
package P31RedefinitionIncompatibleDirection {
    port def OutPort {
        out ref anout;
    }
    part p {
        port aport : OutPort {
            in ref redefines anout;
        }
    }
}
