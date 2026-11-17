using OpenMBEE.OpenSysML.LoadContextProbe;

// Usage:
//   dispose <wire> <binary>     three copies each open two connections, then each closes them and is unloaded
//   unload <wire> <binary>      three copies each open two connections, then each is unloaded with them open
//   unload-starting <wire> <binary> <pid-file> <binary> <pid-file> <binary> <pid-file>
//                               each copy starts a service that never reports an address, and is unloaded while it waits
// <wire> is grpc, connect-proto or connect-json. Each step is written as a line of JSON; the last says
// whether every copy was collected. The probe then returns from Main, so a foreground thread left
// running keeps it from exiting.
return Host.Run(args);
