// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.generator;

import com.nomagic.magicdraw.actions.MDAction;
import java.util.List;
import org.openmbee.mdk.docgen.docbook.DocumentElement;

public interface Generatable {
  void initialize();
  void parse();
  List<DocumentElement> visit(boolean forViewEditor, String outputDir);
  List<MDAction> getActions();
}
