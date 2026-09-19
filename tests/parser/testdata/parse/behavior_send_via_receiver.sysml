action Sender {
    port p : Port;
    action sendIt {
        send data via p to receiver;
    }
}
