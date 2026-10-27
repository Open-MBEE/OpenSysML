package org.openmbee.mdk.syson.run;

import java.util.List;

public record RunTableRow(String element, String siriusId, List<String> cells) {
    public RunTableRow {
        cells = List.copyOf(cells);
    }
}
