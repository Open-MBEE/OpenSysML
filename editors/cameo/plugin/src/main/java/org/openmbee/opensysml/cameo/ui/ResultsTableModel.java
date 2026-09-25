package org.openmbee.opensysml.cameo.ui;

import java.util.ArrayList;
import java.util.List;
import javax.swing.table.AbstractTableModel;
import org.openmbee.opensysml.cameo.results.RunResult.Outcome;

/** Read-only outcome rows: element, status, detail. */
public final class ResultsTableModel extends AbstractTableModel {
  private static final String[] COLUMNS = {"Element", "Status", "Detail"};
  private final List<Outcome> rows = new ArrayList<>();

  public void set(List<Outcome> outcomes) {
    rows.clear();
    rows.addAll(outcomes);
    fireTableDataChanged();
  }

  public Outcome outcome(int row) {
    return rows.get(row);
  }

  @Override
  public int getRowCount() {
    return rows.size();
  }

  @Override
  public int getColumnCount() {
    return COLUMNS.length;
  }

  @Override
  public String getColumnName(int column) {
    return COLUMNS[column];
  }

  @Override
  public Object getValueAt(int row, int column) {
    Outcome outcome = rows.get(row);
    return switch (column) {
      case 0 -> outcome.label();
      case 1 -> outcome.status();
      default -> outcome.detail();
    };
  }
}
