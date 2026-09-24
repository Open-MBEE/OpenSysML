// Streaming flow: a producer's nested step writes its out pin, which streams at
// once to the in pin of a consumer under way beside it; the consumer's nested
// steps read that pin, so they do not commute with the write — the pins are named
// differently and each held per performance, so only the flow relates them. The
// consumer waits for the producer's word before completing, so no value comes late.
package test {
	private import ScalarValues::*;

	action stream {
		attribute seen : Integer = -1;
		attribute final : Integer = -1;
		first start;
		fork split;
		action producer {
			out value : Integer = 0;
			first start;
			action emit { assign value := 7; }
			action tell send 1;
			done;
			succession first start then emit;
			succession first emit then tell;
			succession first tell then done;
		}
		action consumer {
			in got : Integer = -1;
			first start;
			action take { assign seen := got; }
			action wait accept go : Integer;
			action retake { assign final := got; }
			done;
			succession first start then take;
			succession first take then wait;
			succession first wait then retake;
			succession first retake then done;
		}
		join sync;
		done;
		succession first start then split;
		succession first split then producer;
		succession first split then consumer;
		succession first producer then sync;
		succession first consumer then sync;
		succession first sync then done;
		flow producer.value to consumer.got;
	}
}
