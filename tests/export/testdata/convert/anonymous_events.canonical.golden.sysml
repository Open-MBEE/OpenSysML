package AnonymousEvents {
    occurrence def Exchange {
        occurrence request;
    }
    part def Sender {
        occurrence exchange : Exchange;
        event exchange.request;
        event occurrence ack;
        event occurrence;
        event occurrence : Exchange;
    }
}
