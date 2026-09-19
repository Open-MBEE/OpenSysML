// The sequence index `seq#(i)` counts from 1, as SequenceFunctions::'#'
// declares (`in index: Positive[1]`): the first element is `#(1)` and the last
// is `#(size(seq))`. Indexing a scalar reads it as the sequence of one element
// KerML views it as, so `bias#(1)` is the bias itself.
package test {
	calc pick {
		in bias : Integer;
		attribute xs : Integer[*] = (10, 20, 30);
		return : Integer = xs#(1) + xs#(2) + xs#(SequenceFunctions::size(xs)) + bias#(1);
	}
}
