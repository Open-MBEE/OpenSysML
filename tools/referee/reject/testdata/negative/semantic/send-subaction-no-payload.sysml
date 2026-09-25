// Invalid: a send that is a state subaction or a transition effect must have a payload (SysML v2 8.3.16 SendActionUsage validateSendActionUsagePayloadArgument; pilot validateSendActionUsagePayloadArgument, which its grammar enforces first: SysML.xtext:1511 SendNodeDeclaration requires the payload).
package SendSubactionNoPayload {
    part def Receiver;
    state def M {
        part target : Receiver;
        state a;
        state b;
        entry send to target;
        transition first a do send to target then b;
    }
}
