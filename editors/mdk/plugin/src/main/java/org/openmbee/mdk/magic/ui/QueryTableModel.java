package org.openmbee.mdk.magic.ui;

import java.util.ArrayList;
import java.util.List;
import javax.swing.table.AbstractTableModel;
import org.openmbee.mdk.magic.results.RunResult.Cell;
import org.openmbee.mdk.magic.results.RunResult.Row;
import org.openmbee.mdk.magic.results.RunResult.Table;

/**
 * Read-only rows of the tables a run answered, one block per query: a leading column names the
 * query, then one column per projected property. Tables of different shapes share the widest
 * header, so a run over several queries still shows as one grid.
 */
public final class QueryTableModel extends AbstractTableModel {
  static final String QUERY_COLUMN = "Query";
  private List<Table> tables = List.of();
  private List<String> columns = List.of(QUERY_COLUMN);
  private List<Row> rows = List.of();
  private List<Table> rowTables = List.of();

  public void set(List<Table> answered) {
    tables = List.copyOf(answered);
    List<String> merged = new ArrayList<>(List.of(QUERY_COLUMN));
    List<Row> flat = new ArrayList<>();
    List<Table> owners = new ArrayList<>();
    for (Table table : tables) {
      for (String column : table.columns()) {
        if (!merged.contains(column)) merged.add(column);
      }
      for (Row row : table.rows()) {
        flat.add(row);
        owners.add(table);
      }
    }
    columns = List.copyOf(merged);
    rows = List.copyOf(flat);
    rowTables = List.copyOf(owners);
    fireTableStructureChanged();
  }

  public boolean isEmpty() {
    return tables.isEmpty();
  }

  /** The element the row at {@code row}, column {@code column} names: the cell's, else the row's; null when none. */
  public String elementId(int row, int column) {
    if (row < 0 || row >= rows.size()) return null;
    Cell cell = cell(row, column);
    if (cell != null && cell.elementId() != null) return cell.elementId();
    return rows.get(row).elementId();
  }

  private Cell cell(int row, int column) {
    if (column <= 0) return null;
    Table table = rowTables.get(row);
    int index = table.columns().indexOf(columns.get(column));
    if (index < 0 || index >= rows.get(row).cells().size()) return null;
    return rows.get(row).cells().get(index);
  }

  @Override
  public int getRowCount() {
    return rows.size();
  }

  @Override
  public int getColumnCount() {
    return columns.size();
  }

  @Override
  public String getColumnName(int column) {
    return columns.get(column);
  }

  @Override
  public Object getValueAt(int row, int column) {
    if (column == 0) return rowTables.get(row).query();
    Cell cell = cell(row, column);
    return cell == null ? "" : cell.text();
  }
}
