// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.docgen.docbook;

import java.util.ArrayList;
import java.util.List;

public class DBTable extends DocumentElement {
  private List<List<DocumentElement>> body = new ArrayList<>();
  private List<List<DocumentElement>> headers = new ArrayList<>();
  private String caption;
  private String style;
  private int cols;
  private boolean showIfEmpty;

  public List<List<DocumentElement>> getBody() { return body; }
  public void setBody(List<List<DocumentElement>> body) { this.body = body; }
  public List<List<DocumentElement>> getHeaders() { return headers; }
  public void setHeaders(List<List<DocumentElement>> headers) { this.headers = headers; }
  public String getCaption() { return caption; }
  public void setCaption(String caption) { this.caption = caption; }
  public String getStyle() { return style; }
  public void setStyle(String style) { this.style = style; }
  public int getCols() { return cols; }
  public void setCols(int cols) { this.cols = cols; }
  public boolean isShowIfEmpty() { return showIfEmpty; }
  public void setShowIfEmpty(boolean showIfEmpty) { this.showIfEmpty = showIfEmpty; }
}
