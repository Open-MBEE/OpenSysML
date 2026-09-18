action def Controller {
    attribute shutDown : ShutDown;
    action interrupt accept :> shutDown;
    action typedPayload accept warning : Warning via alarmPort;
    action bodied accept :> shutDown {
        assign count := count + 1;
    }
}
