package RenderViewDemo {
    private import Views::*;
    private import StandardViewDefinitions::*;

    port def API;

    part def Sender {
        port api : API;
        port spare : API;
    }

    part def Receiver {
        port api : API;
    }

    interface def Link {
        end client : API;
        end store : ~API;
    }

    part def Assembly {
        part sender : Sender;
        part receiver : Receiver;
        interface link : Link connect client ::> sender.api to store ::> receiver.api;
    }

    view connections {
        expose Assembly;
        render asInterconnectionDiagram;
    }
}
