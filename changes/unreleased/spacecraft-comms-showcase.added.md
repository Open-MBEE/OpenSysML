- **A fifth runtime-showcase model: a spacecraft downlink.** `examples/runtime-showcase/spacecraft-comms.sysml`
  re-spells the OpenSE Cookbook's Spacecraft Example in current SysML v2: a ground station and a
  spacecraft with conjugate ports on a `CommunicationLink` interface, a `parallel` state machine
  whose `dataTransit` region sends frames and drains the battery in a forked do action while its
  `charging` region recharges on a change trigger, a `BatteryLow` signal that interrupts the
  transmission and a change trigger that resumes it. The walkthrough runs both parts on one clock
  with `%advance`, reads values off either object with `%eval in <object> : <expr>`, and shows
  where three timers falling due at the same instant leave `-schedule` a choice — 49 or 50 frames
  before the first interruption — while every schedule reaches the same end: 100 frames received
  at t=241. The REPL tests pin those checkpoints under `reverse`, `declared` and two seeds.
