- **Closing a Node client connection now cancels the calls still under way on it.** They fail with
  `ClosedConnectionError` at once instead of staying on the wire until their timeout, or being cut off
  when the last `close()` stops the private service. A WebAssembly worker also no longer starts a call
  that was cancelled or timed out while it waited behind another; a call Go has already started still
  runs to completion, which the client's README now says.
