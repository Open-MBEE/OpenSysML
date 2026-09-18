package test {
	state def Counting {
		attribute count : Integer = 1;
		entry; then active;
		state active {
			entry action increment {
				assign count := count + 1;
			}
		}
	}

	part def ConfiguredCounter {
		exhibit state modes : Counting {
			attribute redefines count = 5;
		}
	}
}
