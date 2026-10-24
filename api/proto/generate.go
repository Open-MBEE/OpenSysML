package proto

// buf reads its configuration from the repository root, so the directive runs
// there; `make proto-buf` pins the buf and plugin versions and regenerates the
// Go stubs only (the Java client generates its classes at Maven build time).
//go:generate sh -c "cd ../.. && make proto-buf"
