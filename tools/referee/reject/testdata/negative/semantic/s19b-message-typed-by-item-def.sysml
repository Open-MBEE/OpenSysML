// Invalid: A message is a flow usage and must be typed by flow definitions, an item definition is not one (SysML 7.16 Flows and Messages, 8.4.12.2; pilot validateFlowUsageType).
package S19bMessageTypedByItemDef {
    item def Note;
    part def Sender {
        event occurrence emit;
    }
    part def Receiver {
        event occurrence receive;
    }
    part def Exchange {
        part sender : Sender;
        part receiver : Receiver;
        message note : Note from sender.emit to receiver.receive;
    }
}
