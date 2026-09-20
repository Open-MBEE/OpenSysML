package org.openmbee.opensysml.syson.run;

public record RunVerdict(String subject, String kind, boolean holds, String detail, String siriusId) {
}
