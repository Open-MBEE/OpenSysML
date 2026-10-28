package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Value;
import org.openmbee.mdk.syson.run.ValueText;

class ValueTextTest {
    @Test
    void rendersPrimitiveAndSequenceValues() {
        assertThat(ValueText.render(new Value.IntegerValue(21))).isEqualTo("21");
        assertThat(ValueText.render(new Value.Sequence(java.util.List.of(new Value.BooleanValue(true)))))
                .isEqualTo("[true]");
    }
}
