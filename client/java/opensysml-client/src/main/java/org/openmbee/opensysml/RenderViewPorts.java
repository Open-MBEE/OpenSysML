package org.openmbee.opensysml;

/** Which view ports the renderer includes. */
public enum RenderViewPorts {
  /** Only ports used by displayed edges. */
  MINIMAL(""),
  /** Every port, including unconnected ports. */
  FULL("full");

  private final String wireName;

  RenderViewPorts(String wireName) {
    this.wireName = wireName;
  }

  String wireName() {
    return wireName;
  }
}
