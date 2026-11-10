package org.openmbee.mdk.magic.actions;

import java.awt.Component;
import java.awt.GridBagConstraints;
import java.awt.GridBagLayout;
import java.awt.Insets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import javax.swing.JLabel;
import javax.swing.JOptionPane;
import javax.swing.JPanel;
import javax.swing.JTextField;
import org.openmbee.mdk.magic.engine.QueryParameter;
import org.openmbee.mdk.magic.engine.QueryPlan;
import org.openmbee.opensysml.DocumentValue;

/** Asks for the bindings of the queries' {@code in} parameters, one field each, read by type ({@link QueryArguments}). */
final class QueryBindingsDialog {
  static final String TITLE = "Magic MDK: Run query";

  private QueryBindingsDialog() {}

  /** Must run on the EDT. Returns the bindings typed, or empty when the user cancelled. */
  static Optional<Map<String, List<DocumentValue>>> ask(Component parent, QueryPlan plan, String selected) {
    Map<String, JTextField> fields = fields(plan, selected);
    JPanel form = form(plan, fields);
    while (true) {
      int choice = JOptionPane.showConfirmDialog(parent, form, TITLE, JOptionPane.OK_CANCEL_OPTION, JOptionPane.QUESTION_MESSAGE);
      if (choice != JOptionPane.OK_OPTION) return Optional.empty();
      try {
        return Optional.of(read(plan, fields));
      } catch (IllegalArgumentException invalid) {
        JOptionPane.showMessageDialog(parent, invalid.getMessage(), TITLE, JOptionPane.ERROR_MESSAGE);
      }
    }
  }

  /** The fields by binding key; the sole element parameter without a value starts out as the selected element. */
  static Map<String, JTextField> fields(QueryPlan plan, String selected) {
    Map<String, JTextField> fields = new LinkedHashMap<>();
    Optional<QueryParameter> sole = plan.soleElementParameter();
    for (QueryParameter parameter : plan.parameters()) {
      JTextField field = new JTextField(32);
      if (sole.isPresent() && sole.get().equals(parameter)) field.setText(selected);
      fields.put(plan.key(parameter), field);
    }
    return fields;
  }

  /** The bindings the fields hold; a blank field binds nothing, which a parameter without a value may not stay. */
  static Map<String, List<DocumentValue>> read(QueryPlan plan, Map<String, JTextField> fields) {
    Map<String, List<DocumentValue>> bindings = new LinkedHashMap<>();
    List<String> missing = new ArrayList<>();
    for (Map.Entry<String, JTextField> entry : fields.entrySet()) {
      QueryParameter parameter = plan.parameter(entry.getKey()).orElseThrow();
      String text = entry.getValue().getText().trim();
      if (text.isEmpty()) {
        if (parameter.required()) missing.add(entry.getKey());
        continue;
      }
      bindings.put(entry.getKey(), QueryArguments.values(text, parameter));
    }
    if (!missing.isEmpty()) {
      throw new IllegalArgumentException("a value is needed for " + String.join(", ", missing));
    }
    return bindings;
  }

  private static JPanel form(QueryPlan plan, Map<String, JTextField> fields) {
    JPanel form = new JPanel(new GridBagLayout());
    GridBagConstraints at = new GridBagConstraints();
    at.insets = new Insets(2, 4, 2, 4);
    at.anchor = GridBagConstraints.WEST;
    at.gridy = 0;
    at.gridx = 0;
    at.gridwidth = 3;
    form.add(new JLabel("Bindings for " + String.join(", ", plan.queries())), at);
    at.gridwidth = 1;
    for (Map.Entry<String, JTextField> entry : fields.entrySet()) {
      QueryParameter parameter = plan.parameter(entry.getKey()).orElseThrow();
      at.gridy++;
      at.gridx = 0;
      form.add(new JLabel(label(entry.getKey(), parameter)), at);
      at.gridx = 1;
      at.fill = GridBagConstraints.HORIZONTAL;
      at.weightx = 1;
      form.add(entry.getValue(), at);
      at.fill = GridBagConstraints.NONE;
      at.weightx = 0;
      at.gridx = 2;
      form.add(new JLabel(parameter.required() ? "required" : "blank keeps the declared value"), at);
    }
    return form;
  }

  static String label(String key, QueryParameter parameter) {
    String type = parameter.type().isBlank() ? "" : " : " + parameter.type();
    return key + type + (parameter.many() ? " [0..*]" : "");
  }
}
