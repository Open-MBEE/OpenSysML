- **The conversion between runtime values and the API's protobuf messages is its own
  package, `internal/frontend/protoconv`.** The instance-graph serialization (`InstanceGraphToProto`,
  `GraphBounds`) and the value conversions in both directions moved there out of the gRPC
  service, so the REPL's `%features … json` and the Go client's value marshalling no longer
  link the service or its Connect transport; `sysml` reaches gRPC-Go only through the
  generated service stubs that share the `api/proto` package with the messages. The
  `features` JSON is unchanged.
