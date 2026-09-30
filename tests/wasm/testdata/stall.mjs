// A runtime that stands in for Node hosting a binary, for the harness's own test of
// its deadline: it announces itself on standard error, then exits when its "binary"
// is named answer and never exits when it is named stall — the shape of a process
// that wrote its output and then deadlocked on the way out.
//
// Usage: node stall.mjs answer|stall [args...]
process.stderr.write(`stall.mjs: started ${process.argv.slice(2).join(' ')}\n`);
if (process.argv[2] === 'answer') {
  process.exit(0);
}
setInterval(() => {}, 1 << 30);
