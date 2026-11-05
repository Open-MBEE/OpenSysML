using OpenMBEE.OpenSysML.CacheProbe;

// Usage:
//   hold <cache-dir>              take the cache lock through the client; report "held", then release it at a
//                                 line or end of input and report "released"
//   try <cache-dir>               try the cache lock through the client once; report whether it was free
//   hold-shared <lock-file>       take the cross-client lock alone with this probe's own system calls, as any
//                                 other client would, waiting for it; report "held", then release it at a line
//                                 or end of input and report "released"; the lock file must exist
//   try-shared <lock-file>        try the cross-client lock alone in the same way; report whether it was free
//   hold-and-start <cache-dir>    take the cache lock through the client, then start this probe in sleep mode,
//                                 sharing its standard streams; report "held" with both ids and wait to be killed
//   sleep                         sleep for two minutes
//   copies <cache-dir>            load two copies of the client into collectible load contexts: copy 0 takes
//                                 the lock, waits for a line, copy 1 tries it, copy 0 releases, copy 1 tries
//                                 again; then both are unloaded and must be collected
//   crash <cache-dir> <step> <payload-file> <version> <repo>
//                                 install the payload as that release in a cache transaction and snapshot it,
//                                 stopping at <step> to report "reached" and wait to be killed
// Each step is written as a line of JSON.
return await Host.RunAsync(args);
