package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Optional;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.DocumentRow;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Quantity;
import org.openmbee.mdk.syson.run.DocumentValueText;

class DocumentValueTextTest {
    @Test
    void rendersCellsAsPlainText() {
        DocumentValue.ElementRef element = new DocumentValue.ElementRef("P::a", "PartUsage");
        assertThat(DocumentValueText.render(List.of(element, new DocumentValue.StringValue("mount"),
                new DocumentValue.IntegerValue(15), new DocumentValue.RealValue(8.5),
                new DocumentValue.BooleanValue(true), new DocumentValue.InfinityValue())))
                .isEqualTo("P::a, mount, 15, 8.5, true, infinity");
        assertThat(DocumentValueText.render(new DocumentValue.ObjectRef(3, "", Optional.empty()))).isEqualTo("#3");
        assertThat(DocumentValueText.render(new DocumentValue.ObjectRef(3, "car.wheels[2]", Optional.empty())))
                .isEqualTo("car.wheels[2] (#3)");
        assertThat(DocumentValueText.render(new DocumentValue.QuantityValue(new Quantity(2.0, Optional.of("kg"), Optional.empty()))))
                .isEqualTo("2.0 kg");
        assertThat(DocumentValueText.render(new DocumentValue.DocumentVerdict(element, "requirement", "text", "path",
                "holds", "", "", List.of()))).isEqualTo("path: holds");
        assertThat(DocumentValueText.render(List.of())).isEmpty();
    }

    @Test
    void labelsRowsByWhatTheyStandFor() {
        DocumentValue.ElementRef type = new DocumentValue.ElementRef("Vehicles::Car", "PartDefinition");
        DocumentValue.ObjectRef car = new DocumentValue.ObjectRef(7, "car", Optional.of(type));
        DocumentRow plain = new DocumentRow(type, List.of(), Optional.empty(), Optional.empty(), Optional.empty(),
                Optional.empty());
        DocumentRow object = new DocumentRow(type, List.of(), Optional.empty(), Optional.of(car), Optional.empty(),
                Optional.empty());
        DocumentRow state = new DocumentRow(type, List.of(), Optional.empty(), Optional.of(car),
                Optional.of(new DocumentValue.DocumentState(car, "machine", "run", "on.run", Optional.empty(), "", List.of("on"))),
                Optional.empty());
        DocumentRow event = new DocumentRow(type, List.of(), Optional.empty(), Optional.of(car), Optional.empty(),
                Optional.of(new DocumentValue.DocumentEvent("transition", new DocumentValue.RealValue(2), "car: off -> on",
                        Optional.of(car), "machine", "", "off", "on", Optional.empty(), "", List.of(), List.of(), "",
                        0L, 0, "", Optional.empty())));
        assertThat(DocumentValueText.rowLabel(plain)).isEqualTo("Vehicles::Car");
        assertThat(DocumentValueText.rowLabel(object)).isEqualTo("car (#7)");
        assertThat(DocumentValueText.rowLabel(state)).isEqualTo("car (#7) in on.run");
        assertThat(DocumentValueText.rowLabel(event)).isEqualTo("car: off -> on");
    }
}
