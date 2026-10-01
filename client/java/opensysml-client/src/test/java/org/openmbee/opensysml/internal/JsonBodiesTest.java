package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.proto.EvaluateResponse;
import org.openmbee.opensysml.proto.ExecuteActionResponse;
import org.openmbee.opensysml.proto.Value;

class JsonBodiesTest {

  private static long bits(double value) {
    return Double.doubleToRawLongBits(value);
  }

  @Test
  void aNegativeZeroKeepsItsSignWhereverItSits() throws Exception {
    EvaluateResponse scalar =
        JsonBodies.parse("{\"result\":{\"realValue\":-0}}", EvaluateResponse.getDefaultInstance());
    assertEquals(bits(-0.0), bits(scalar.getResult().getRealValue()));

    EvaluateResponse nested =
        JsonBodies.parse(
            "{\"result\":{\"sequence\":{\"elements\":[{\"realValue\":1},{\"realValue\":-0.0}]}}}",
            EvaluateResponse.getDefaultInstance());
    assertEquals(bits(1.0), bits(nested.getResult().getSequence().getElements(0).getRealValue()));
    assertEquals(
        bits(-0.0), bits(nested.getResult().getSequence().getElements(1).getRealValue()));

    ExecuteActionResponse mapped =
        JsonBodies.parse(
            "{\"outputs\":{\"a\":{\"realValue\":-0},\"b\":{\"realValue\":0},"
                + "\"c\":{\"complex\":{\"real\":-0e0,\"imaginary\":\"-0\"}}}}",
            ExecuteActionResponse.getDefaultInstance());
    assertEquals(bits(-0.0), bits(mapped.getOutputsOrThrow("a").getRealValue()));
    assertEquals(bits(0.0), bits(mapped.getOutputsOrThrow("b").getRealValue()));
    assertEquals(bits(-0.0), bits(mapped.getOutputsOrThrow("c").getComplex().getReal()));
    assertEquals(bits(-0.0), bits(mapped.getOutputsOrThrow("c").getComplex().getImaginary()));
  }

  @Test
  void aBodyWithoutANegativeZeroReadsAsJsonFormatReadsIt() throws Exception {
    Value want = Value.newBuilder().setRealValue(-0.5).build();
    EvaluateResponse read =
        JsonBodies.parse(
            "{\"result\":{\"realValue\":-0.5},\"error\":\"-0\"}",
            EvaluateResponse.getDefaultInstance());
    assertEquals(want, read.getResult());
    assertEquals("-0", read.getError());
  }
}
