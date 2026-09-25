package Conjugation {
    port def CommunicationPort {
        out item signal;
    }
    part def Ground {
        port groundPort : CommunicationPort;
        port spacePort : ~CommunicationPort;
    }
    interface def Link {
        end port a : CommunicationPort;
        end port b : ~CommunicationPort;
    }
}
