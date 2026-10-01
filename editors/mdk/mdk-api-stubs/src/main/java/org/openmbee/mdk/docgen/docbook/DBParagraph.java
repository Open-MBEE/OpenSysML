// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.docgen.docbook;

public class DBParagraph extends DocumentElement {
  private Object text;

  public DBParagraph() {}
  public DBParagraph(Object text) { this.text = text; }
  public void setText(Object text) { this.text = text; }
  public Object getText() { return text; }
}
