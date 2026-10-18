package org.openmbee.opensysml.syson.run;

import java.util.List;

public record RunTable(List<String> columns, List<RunTableRow> rows) {
    public RunTable {
        columns = List.copyOf(columns);
        rows = List.copyOf(rows);
    }
}
