package org.openmbee.mdk.syson.run;

import java.util.List;

public record RunInstance(long id, String type, String typeSiriusId, List<RunNamedValue> featureValues) {
}
