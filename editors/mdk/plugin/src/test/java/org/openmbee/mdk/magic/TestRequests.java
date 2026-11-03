package org.openmbee.mdk.magic;

import java.util.List;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.source.V2TextualSource;

final class TestRequests {
  private TestRequests() {}

  static RunRequest request(Operation operation, String subject) {
    return new RunRequest(operation, new V2TextualSource("package Demo;"), subject, List.of());
  }
}
