package org.openmbee.opensysml.cameo;

import java.util.List;
import org.openmbee.opensysml.cameo.engine.Operation;
import org.openmbee.opensysml.cameo.engine.RunRequest;
import org.openmbee.opensysml.cameo.source.V2TextualSource;

final class TestRequests {
  private TestRequests() {}

  static RunRequest request(Operation operation, String subject) {
    return new RunRequest(operation, new V2TextualSource("package Demo;"), subject, List.of());
  }
}
