package P {
	item def Ball;
	action def Run {
		in feed : ScalarValues::Integer;
		private attribute count : ScalarValues::Integer[0..*] ordered;
		private ref item ball : Ball;
		private ref any;
		action add {
			in value : ScalarValues::Integer;
			assign count := SequenceFunctions::including(SequenceFunctions::excluding(count, value), value);
		}
		action addAt {
			in value : ScalarValues::Integer;
			in insertAt : ScalarValues::Integer;
			assign count := SequenceFunctions::includingAt(count, value, insertAt);
		}
		action removeAt {
			in removeAt : ScalarValues::Integer;
			assign count := SequenceFunctions::excludingAt(count, removeAt);
		}
		action remove {
			in value : ScalarValues::Integer;
			assign count := SequenceFunctions::excluding(count, value);
		}
		action clear {
			assign count := null;
		}
		action read {
			out result : ScalarValues::Integer[0..*] = count;
		}
		action reduce {
			in collection : ScalarValues::Integer[0..*];
			out result : ScalarValues::Integer = collection->ControlFunctions::reduce {in x; in y; IntegerFunctions::'+'(x, y)};
		}
		action raise {
			in exception : ScalarValues::Integer;
		}
		action 'terminate' terminate;
		first raise then 'terminate';
	}
}
