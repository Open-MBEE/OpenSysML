package F65 {
	item def Publish;

	connection def AB {
		end a;
		end b;
	}

	part def P {
		part a;
		part b;

		bind a = b;
		binding ab bind a = b;
		binding ab1 : AB bind a = b;
		binding : AB bind a = b;
		binding ab2 : AB [1] bind a = b;

		part producer[1] {
			event occurrence publish_source_event;
		}
		part server[1] {
			event occurrence publish_target_event;
		}

		message m1 of Publish[1];
		message m2 of [1] Publish;
		message m3 of payload : Publish[1];
		message m4 of Publish from producer.publish_source_event to server.publish_target_event;
	}

	part def Q :> P {
		part :>> producer {
			event publish_source_event = m4.start;
		}
		part :>> server {
			event occurrence :>> publish_target_event = m4.done;
		}
	}
}
