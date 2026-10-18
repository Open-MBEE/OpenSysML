// Messages ordered by the successions on the lifelines their events are on,
// whichever lifeline each message is received on.
package Signals {
	item def A;
	item def B;
	item def C;
}

package Lifelines {
	private import Signals::*;

	// q receives a, then sends b, then sends c; the receive events of b and c
	// are on two lifelines nothing orders against each other.
	occurrence def Relay {
		part p {
			event occurrence s1;
			then event occurrence r3;
		}

		part q {
			event occurrence r1;
			then event occurrence s2;
			then event occurrence s3;
		}

		part r {
			event occurrence r2;
		}

		message a of A from p.s1 to q.r1;
		message b of B from q.s2 to r.r2;
		message c of C from q.s3 to p.r3;
	}

	// The same interaction with its lifelines declared in the other order.
	occurrence def RelayReversed {
		part r {
			event occurrence r2;
		}

		part q {
			event occurrence r1;
			then event occurrence s2;
			then event occurrence s3;
		}

		part p {
			event occurrence s1;
			then event occurrence r3;
		}

		message a of A from p.s1 to q.r1;
		message b of B from q.s2 to r.r2;
		message c of C from q.s3 to p.r3;
	}

	// The only order runs through `settle`, an event no message attaches to.
	occurrence def Settled {
		part client {
			event occurrence askSent;
			event occurrence replyTaken;
		}

		part server {
			event occurrence askTaken;
			then event occurrence settle;
			then event occurrence replySent;
		}

		message reply of B from server.replySent to client.replyTaken;
		message ask of A from client.askSent to server.askTaken;
	}

	// Two messages nothing orders: `pong` is declared first and stays first.
	occurrence def Unordered {
		part left {
			event occurrence pongSent;
			event occurrence pingSent;
		}

		part right {
			event occurrence pongTaken;
			event occurrence pingTaken;
		}

		message pong of B from left.pongSent to right.pongTaken;
		message ping of A from left.pingSent to right.pingTaken;
	}

	// A succession ordering a message's receive before its send: no row can
	// hold both, so the cycle is reported and declaration order stands.
	occurrence def Reversed {
		part p {
			event occurrence sent;
			event occurrence other;
		}

		part q {
			event occurrence received;
			event occurrence otherTaken;
		}

		message back of B from p.other to q.otherTaken;
		message m of A from p.sent to q.received;
		succession first q.received then p.sent;
	}

	// The events can be ordered but the rows cannot: each succession puts
	// `one` on the other side of `two`.
	occurrence def Crossed {
		part p {
			event occurrence firstSent;
			event occurrence secondTaken;
		}

		part q {
			event occurrence firstTaken;
			event occurrence secondSent;
		}

		message two of B from q.secondSent to p.secondTaken;
		message one of A from p.firstSent to q.firstTaken;
		succession first p.firstSent then p.secondTaken;
		succession crossing first q.secondSent then q.firstTaken;
	}

	// Messages attached to the lifelines themselves have no time of their own:
	// two running opposite ways order nothing; the other two's order still holds.
	occurrence def Reciprocal {
		part p {
			event occurrence lateSent;
			event occurrence earlyTaken;
		}

		part q {
			event occurrence lateTaken;
			event occurrence earlySent;
		}

		message forward of A from p to q;
		message backward of B from q to p;
		message late of C from p.lateSent to q.lateTaken;
		message early of A from q.earlySent to p.earlyTaken;
		succession first p.earlyTaken then p.lateSent;
	}
}

package LifelineViews {
	private import StandardViewDefinitions::*;

	view relayView : SequenceView {
		expose Lifelines::Relay;
	}

	view relayReversedView : SequenceView {
		expose Lifelines::RelayReversed;
	}

	view settledView : SequenceView {
		expose Lifelines::Settled;
	}

	view unorderedView : SequenceView {
		expose Lifelines::Unordered;
	}

	view reversedView : SequenceView {
		expose Lifelines::Reversed;
	}

	view crossedView : SequenceView {
		expose Lifelines::Crossed;
	}

	view reciprocalView : SequenceView {
		expose Lifelines::Reciprocal;
	}
}
