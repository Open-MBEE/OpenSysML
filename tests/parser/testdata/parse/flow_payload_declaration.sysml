package MessagePayload {
	item def FuelCommand {
		attribute fuelFlow;
	}

	occurrence def Interaction {
		ref part sender;
		ref part receiver;

		message fuelCommandMessage of fuelCommand : FuelCommand
			from sender.sent to receiver.received;

		message forwardMessage of fuelCommand : FuelCommand = fuelCommandMessage.fuelCommand
			from sender.forwarded to receiver.received;

		flow plainPayload of FuelCommand from sender.sent to receiver.received;
	}
}
