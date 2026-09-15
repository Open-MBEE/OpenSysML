- **Occurrences of one shape share their derived defaults and their verdicts.** A `=` default
  that one pristine occurrence of a type derives from nothing but declared values under itself
  is now recorded against the occurrence's shape — its type, classifiers and holding feature — in
  a side table of the runtime context, and every other pristine occurrence of that shape reads
  the recorded value instead of deriving it again and materializing the component tree the
  derivation walked; an occurrence that states, writes, binds or classifies anything the
  derivation read derives on its own, and a write under an occurrence invalidates what it took.
  Within one `-satisfy` or `-validate=<object>` report, a check over occurrences of one shape is
  evaluated once per distinct set of inputs and its verdict fanned out to each occurrence, which
  still reports its own verdict, message and path in the same order. Values, verdicts and
  diagnostics are unchanged, as `TestSparseValuesDifferential` asserts with sharing on and off
  (`OPENSYSML_SHARED_DEFAULTS=0` turns it off). On the 12 800-satellite fleet constellation,
  checking its 2 412 satisfaction assertions drops from 23.2 s and 14.4 GiB allocated to 10.9 s
  and 6.1 GiB, and reading one summed attribute over every occurrence from 259 s and 74.3 GiB to
  8.6 s and 2.1 GiB.
