// Census: validateFlowEndImplicitSubsetting
// Expect: error: Must be an accessible feature (use dot notation for nesting)
package P {
  part def C {
    part a { out attribute o : ScalarValues::Integer; }
    part b { in attribute i : ScalarValues::Integer; }
    flow f from a::o to b::i;
  }
}
