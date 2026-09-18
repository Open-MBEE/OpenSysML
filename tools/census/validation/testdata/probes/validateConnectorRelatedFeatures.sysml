// Census: validateConnectorRelatedFeatures
// Expect: error: Must have at least two related elements
package P {
  part def B;
  part b0 : B;
  part p {
    connection { end ::> b0; }
  }
}
