package org.openmbee.opensysml.conformance;

import com.google.gson.annotations.SerializedName;
import java.util.ArrayList;
import java.util.List;

/**
 * The machine-readable result of a run, in the shape {@code tools/cmd/conformance} writes so the two are
 * comparable.
 */
final class Report {

  /** The path of the service the run tested. */
  String service = "";

  int total;
  int passed;
  int failed;
  int skipped;
  int errored;
  List<Summary> protocols = new ArrayList<>();

  /** One protocol's results. */
  static final class Summary {
    String protocol;
    String service;
    List<String> capabilities;
    int total;
    int passed;
    int failed;
    int skipped;
    int errored;
    List<Result> results = new ArrayList<>();

    Summary(String protocol, String service, List<String> capabilities) {
      this.protocol = protocol;
      this.service = service;
      this.capabilities = capabilities;
    }
  }

  /** One scenario's outcome. */
  static final class Result {
    String id;
    /** pass, fail, skip or error; an error is the scenario itself being wrong. */
    String outcome = "pass";

    String rpc;
    /** Why it was skipped, or what it disagreed about. */
    String reason;

    List<String> failures;
    /** The status the call answered, as a code name. */
    String status = "OK";

    @SerializedName("duration_ms")
    double durationMs;

    Result(String id, String rpc) {
      this.id = id;
      this.rpc = rpc;
    }
  }

  /** Adds a protocol's results to the totals. */
  void add(Summary summary) {
    protocols.add(summary);
    total += summary.total;
    passed += summary.passed;
    failed += summary.failed;
    skipped += summary.skipped;
    errored += summary.errored;
  }
}
