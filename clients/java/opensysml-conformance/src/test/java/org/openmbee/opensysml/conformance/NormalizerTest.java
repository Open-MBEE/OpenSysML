package org.openmbee.opensysml.conformance;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.openmbee.opensysml.proto.Diagnostic;
import org.openmbee.opensysml.proto.EvaluateResponse;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.Function;
import org.openmbee.opensysml.proto.Instance;
import org.openmbee.opensysml.proto.InstantiateResponse;
import org.openmbee.opensysml.proto.ParseFileResponse;
import org.openmbee.opensysml.proto.ServerInfoResponse;
import org.openmbee.opensysml.proto.Span;
import org.openmbee.opensysml.proto.Value;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

/** The values a normalized answer replaces, so two runs of the same scenario compare equal. */
class NormalizerTest {

  @Test
  void onlySetFieldsAppear() {
    Map<String, Object> normalized =
        new Normalizer("").normalize(ParseFileResponse.newBuilder().setModelHash("abc").build());
    assertEquals(Map.of("model_hash", "abc"), normalized);
  }

  @Test
  void theServiceVersionIsAPlaceholder() {
    Map<String, Object> normalized =
        new Normalizer("")
            .normalize(
                ServerInfoResponse.newBuilder().setVersion("0.9.1").addCapabilities("query").build());
    assertEquals(Normalizer.VERSION, normalized.get("version"));
    assertEquals(List.of("query"), normalized.get("capabilities"));
  }

  @Test
  void theModelHashTheCallCarriedIsAPlaceholder() {
    Map<String, Object> normalized =
        new Normalizer("deadbeef")
            .normalize(ParseFileResponse.newBuilder().setModelHash("deadbeef").build());
    assertEquals(Normalizer.MODEL_HASH, normalized.get("model_hash"));
  }

  @Test
  void anAbsolutePathIsAPlaceholderAndARelativeOneIsNot() {
    Map<String, Object> absolute =
        new Normalizer("")
            .normalize(
                ParseFileResponse.newBuilder()
                    .addDiagnostics(
                        Diagnostic.newBuilder().setSpan(Span.newBuilder().setFile("/tmp/model.sysml")))
                    .build());
    Map<String, Object> relative =
        new Normalizer("")
            .normalize(
                ParseFileResponse.newBuilder()
                    .addDiagnostics(
                        Diagnostic.newBuilder().setSpan(Span.newBuilder().setFile("<content>")))
                    .build());
    assertEquals(Normalizer.PATH, span(absolute).get("file"));
    assertEquals("<content>", span(relative).get("file"));
  }

  @Test
  void instanceIdsAreLabelledInOrderOfFirstAppearance() {
    InstantiateResponse response =
        InstantiateResponse.newBuilder()
            .setInstance(
                Instance.newBuilder()
                    .setId(41)
                    .putFeatureValues(
                        "part",
                        FeatureValue.newBuilder()
                            .setFeatureName("part")
                            .setValue(Value.newBuilder().setInstanceId(7))
                            .build())
                    .putFeatureValues(
                        "scale",
                        FeatureValue.newBuilder()
                            .setFeatureName("scale")
                            .setValue(
                                Value.newBuilder()
                                    .setFunction(
                                        Function.newBuilder().setCalcId("T::scale").setSelfId(41)))
                            .build()))
            .addInstances(Instance.newBuilder().setId(41))
            .addInstances(Instance.newBuilder().setId(7))
            .build();
    Map<String, Object> normalized = new Normalizer("").normalize(response);

    Map<?, ?> root = (Map<?, ?>) normalized.get("instance");
    assertEquals("@1", root.get("id"));
    Map<?, ?> features = (Map<?, ?>) root.get("feature_values");
    Map<?, ?> value = (Map<?, ?>) ((Map<?, ?>) features.get("part")).get("value");
    assertEquals("@2", value.get("instance_id"));
    Map<?, ?> function =
        (Map<?, ?>) ((Map<?, ?>) ((Map<?, ?>) features.get("scale")).get("value")).get("function");
    assertEquals("@1", function.get("self_id"));

    List<?> instances = (List<?>) normalized.get("instances");
    assertEquals("@1", ((Map<?, ?>) instances.get(0)).get("id"));
    assertEquals("@2", ((Map<?, ?>) instances.get(1)).get("id"));
  }

  @Test
  void aRealArrivesAsADouble() {
    Map<String, Object> normalized =
        new Normalizer("")
            .normalize(
                EvaluateResponse.newBuilder()
                .setResult(Value.newBuilder().setRealValue(2.5))
                .build());
    assertEquals(2.5, ((Map<?, ?>) normalized.get("result")).get("real_value"));
  }

  private static Map<?, ?> span(Map<String, Object> normalized) {
    Map<?, ?> diagnostic = (Map<?, ?>) ((List<?>) normalized.get("diagnostics")).get(0);
    return (Map<?, ?>) diagnostic.get("span");
  }
}
