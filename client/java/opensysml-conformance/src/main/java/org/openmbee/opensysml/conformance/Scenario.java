package org.openmbee.opensysml.conformance;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import java.util.List;
import java.util.Optional;

/**
 * One conformance case: a call to make and what it must answer.
 *
 * @param id unique id, which a report and {@code -run} address it by
 * @param description why it is part of the contract
 * @param rpc method name, bare or qualified
 * @param requiresCapabilities names {@code GetServerInfo} must report for {@code expect} to apply
 * @param expectWithoutCapability what a service reporting none of them must answer instead
 * @param model the fixture parsed before the call, when it needs one
 * @param request the request as protobuf JSON, with placeholders unresolved
 * @param expect what the answer must be
 * @param file the scenario file it came from
 */
record Scenario(
    String id,
    String description,
    String rpc,
    List<String> requiresCapabilities,
    Optional<Expect> expectWithoutCapability,
    Optional<Fixture> model,
    JsonObject request,
    Expect expect,
    String file) {

  /**
   * The source a scenario needs parsed before its call: one fixture, or several parsed together
   * as one model, which the public API's single-document {@code parse} cannot make.
   */
  record Fixture(
      String fixture, List<String> fixtures, String language, boolean strictConformance) {}

  /** The bare method name, whether the scenario qualified it or not. */
  String method() {
    int separator = rpc.lastIndexOf('/');
    return separator < 0 ? rpc : rpc.substring(separator + 1);
  }

  /**
   * What a call must answer. Every member is optional and all of them must hold.
   *
   * @param status canonical status name, empty when the call must succeed
   * @param statusMessageContains substring the status message must contain
   * @param response tree compared field by field against the answer
   * @param contains path to a substring its text must contain
   * @param containsAll path to strings all of which must be there
   * @param nonEmpty paths that must hold a value other than their default
   * @param absent paths that must be unset or hold their default
   * @param counts path to the exact number of entries there
   * @param minCounts path to a lower bound on that number
   */
  record Expect(
      String status,
      String statusMessageContains,
      Optional<JsonObject> response,
      java.util.Map<String, String> contains,
      java.util.Map<String, List<String>> containsAll,
      List<String> nonEmpty,
      List<String> absent,
      java.util.Map<String, JsonElement> counts,
      java.util.Map<String, JsonElement> minCounts) {

    /** The status this expectation names, which is {@code OK} when it names none. */
    String wantStatus() {
      return status.isEmpty() ? "OK" : status;
    }
  }
}
