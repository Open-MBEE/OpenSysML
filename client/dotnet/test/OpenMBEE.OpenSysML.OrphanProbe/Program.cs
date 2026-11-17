using OpenMBEE.OpenSysML.OrphanProbe;

// Usage:
//   sibling                           an unrelated process: ignores its input and sleeps, two minutes at most
//   serve <binary> <wire> <siblings>  opens through Connection.OpenAsync, then starts the siblings one by one
//   race <binary> <wire> <siblings>   starts each sibling on its own thread the moment the service starts
//   leak <binary> <wire>              the leak control: one sibling inherits a duplicate of the service's input
// <wire> is grpc, connect-proto or connect-json. Every mode but sibling writes one line of JSON,
// {"probe":…,"service":…,"address":…,"siblings":[…]}, the address being where the probe reaches the
// service, then waits to be killed; it exits by itself only when its own input ends, so a test host
// that dies leaves no probe behind.
return await Probe.RunAsync(args);
