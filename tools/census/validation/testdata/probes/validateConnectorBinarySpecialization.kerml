// Census: validateConnectorBinarySpecialization
// Expect: error: has 3 ends but specializes a binary link
package P {
  class T {
    feature x : T; feature y : T; feature z : T;
    connector c : Links::BinaryLink (x, y, z);
  }
}
