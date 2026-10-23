// The publish/subscribe interaction of the OMG sequence-modeling validation model,
// run as the transfers its messages are realized by: each message is posted from
// the lifeline holding the event it leaves to the lifeline holding the event it
// reaches, in the order the lifelines' successions admit — the subscription first,
// although the publication is declared first.
package Sequences {
	item def Subscribe {
		attribute topic;
	}
	item def Publish {
		attribute topic;
	}
	item def Deliver;

	occurrence def PubSubSequence {
		part producer[1] {
			event occurrence publish_source_event;
		}

		message publish_message of Publish[1] from producer.publish_source_event to server.publish_target_event;

		part server[1] {
			event occurrence subscribe_target_event;
			then event occurrence publish_target_event;
			then event occurrence deliver_source_event;
		}

		message subscribe_message of Subscribe[1] from consumer.subscribe_source_event to server.subscribe_target_event;
		message deliver_message of Deliver[1] from server.deliver_source_event to consumer.deliver_target_event;

		part consumer[1] {
			event occurrence subscribe_source_event;
			then event occurrence deliver_target_event;
		}
	}
}
