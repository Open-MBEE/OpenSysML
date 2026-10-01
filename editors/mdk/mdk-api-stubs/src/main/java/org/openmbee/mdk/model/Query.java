// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.model;

import com.nomagic.magicdraw.actions.MDAction;
import java.util.ArrayList;
import java.util.List;
import org.openmbee.mdk.docgen.docbook.DocumentElement;
import org.openmbee.mdk.generator.Generatable;

public abstract class Query extends DocGenElement implements Generatable {
  protected List<Object> targets;
  protected List<String> titles;
  protected boolean sortElementsByName = false;

  public void setTargets(List<Object> targets) { this.targets = targets; }
  public List<Object> getTargets() { return targets; }
  public void setTitles(List<String> titles) { this.titles = titles; }
  public List<String> getTitles() { return titles; }
  public boolean isSortElementsByName() { return sortElementsByName; }
  public void setSortElementsByName(boolean sortElementsByName) { this.sortElementsByName = sortElementsByName; }

  @Override
  public void initialize() {}

  @Override
  public List<DocumentElement> visit(boolean forViewEditor, String outputDir) { return new ArrayList<>(); }

  @Override
  public void parse() {}

  @Override
  public List<MDAction> getActions() { return new ArrayList<>(); }
}
