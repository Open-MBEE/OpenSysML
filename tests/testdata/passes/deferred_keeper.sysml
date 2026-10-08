package DeferredKeeper {
	item def Ping;
	item def Go;

	state def Legacy {
		entry; then busy;
		state busy {
			@MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
			item deferred : Ping[*] ordered;
			do action buffer {
				first start then receive;
				action receive accept kept : Ping;
				then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
				then receive;
			}
			exit action flush {
				for kept in deferred { send kept to self; }
				then action clear { assign deferred := (); }
			}
		}
		state ready;
		transition first busy accept Go then ready;
	}

	state def Marked {
		entry; then busy;
		state busy {
			@MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
			item deferred : Ping[*] ordered;
			do action buffer {
				first start then receive;
				#MigrationMetadata::DeferredKeeper action receive accept kept : Ping;
				then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
				then receive;
			}
			exit action flush {
				for kept in deferred { send kept to self; }
				then action clear { assign deferred := (); }
			}
		}
		state ready;
		transition first busy accept Go then ready;
	}

	state def Undeferring {
		entry; then busy;
		state busy {
			do action work {
				action receive accept kept : Ping;
			}
		}
		state ready;
		transition first busy accept Go then ready;
	}
}
