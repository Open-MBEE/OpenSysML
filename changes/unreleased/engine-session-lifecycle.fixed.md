- **An external engine's answer is no longer lost or left blocking when its process ends or answers twice.**
  An answer the engine wrote just before it exited is returned instead of sometimes being reported as
  the process exiting; a second response to one request ends the session as a protocol break instead of
  blocking the reader for good; collecting the exit no longer closes the engine's standard output under
  the reader; and starting an engine stops with its caller's context instead of running out
  `OPENSYSML_TOOL_TIMEOUT`, without ending the engine for the rest of the plan.
