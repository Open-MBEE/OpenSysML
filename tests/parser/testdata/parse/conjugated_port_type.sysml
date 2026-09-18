// A port typed by the conjugate of a port definition (SysML v2 8.2.2.12.1):
// `~P` names the conjugated definition P declares, and conjugation composes.
package ConjugatedPorts {
    port def CommunicationPort {
        in item command;
        out item telemetry;
        inout item clock;
    }

    interface def CommunicationInterface {
        end groundPort : CommunicationPort;
        end spacePort : ~CommunicationPort;
    }

    port def RelayPort {
        port inner : ~CommunicationPort;
    }

    part def Radio {
        port rx : ~CommunicationPort;
        port relay : ~RelayPort;
        port explicit : CommunicationPort::'~CommunicationPort';
    }
}
