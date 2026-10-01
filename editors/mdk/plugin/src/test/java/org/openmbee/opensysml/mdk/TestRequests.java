package org.openmbee.opensysml.mdk;

import java.util.List;
import org.openmbee.opensysml.mdk.engine.Operation;
import org.openmbee.opensysml.mdk.engine.RunRequest;
import org.openmbee.opensysml.mdk.source.V2TextualSource;

final class TestRequests {
  private TestRequests() {}

  static RunRequest request(Operation operation, String subject) {
    return new RunRequest(operation, new V2TextualSource("package Demo;"), subject, List.of());
  }
}
