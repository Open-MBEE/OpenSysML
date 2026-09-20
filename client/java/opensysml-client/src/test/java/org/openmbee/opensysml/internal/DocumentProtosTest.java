package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.DocumentRow;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.proto.RunDocumentQueryResponse;

/** Document-query values read off the wire and written back, and rows spread by their element. */
class DocumentProtosTest {

  private static DocumentValue roundTrip(org.openmbee.opensysml.proto.DocumentValue proto) {
    return Protos.documentValue(Protos.proto(Protos.documentValue(proto)));
  }

  @Test
  void anElementValueCarriesItsNameAndTypeBothDirections() {
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
            .setElementId("Observatory::telescope")
            .setElementType("PartUsage")
            .build();
    DocumentValue value = Protos.documentValue(proto);
    assertEquals(new DocumentValue.ElementRef("Observatory::telescope", "PartUsage"), value);
    assertEquals(proto, Protos.proto(value));
  }

  @Test
  void anAnonymousElementIsAReferenceOfTypeAlone() {
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
            .setElementType("PartUsage")
            .build();
    assertEquals(new DocumentValue.ElementRef("", "PartUsage"), Protos.documentValue(proto));
    assertEquals(proto, Protos.proto(new DocumentValue.ElementRef("", "PartUsage")));
  }

  @Test
  void everyLiteralKindRoundTrips() {
    assertEquals(
        new DocumentValue.StringValue("s"),
        roundTrip(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                .setStringValue("s")
                .build()));
    assertEquals(
        new DocumentValue.IntegerValue(7),
        roundTrip(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder().setIntValue(7).build()));
    assertEquals(
        new DocumentValue.RealValue(1.5),
        roundTrip(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder().setRealValue(1.5).build()));
    assertEquals(
        new DocumentValue.BooleanValue(true),
        roundTrip(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder().setBoolValue(true).build()));
    assertEquals(
        new DocumentValue.InfinityValue(),
        roundTrip(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder().setInfinity(true).build()));
  }

  @Test
  void aQuantityValueRoundTripsThroughTheQuantityMapping() {
    Quantity quantity = new Quantity(2.5, Optional.empty(), Optional.empty());
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
            .setQuantity(
                org.openmbee.opensysml.proto.Quantity.newBuilder().setRealMagnitude(2.5))
            .build();
    DocumentValue value = Protos.documentValue(proto);
    assertEquals(new DocumentValue.QuantityValue(quantity), value);
    assertEquals(proto, Protos.proto(value));
  }

  @Test
  void anObjectValueCarriesIdPathAndElement() {
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
            .setObject(
                org.openmbee.opensysml.proto.DocumentObject.newBuilder()
                    .setInstanceId(2)
                    .setPath("Garage::car.wheels[2]")
                    .setElement(
                        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                            .setElementId("Garage::car::wheels")
                            .setElementType("PartUsage")))
            .build();
    DocumentValue value = Protos.documentValue(proto);
    assertEquals(
        new DocumentValue.ObjectRef(
            2, "Garage::car.wheels[2]",
            Optional.of(new DocumentValue.ElementRef("Garage::car::wheels", "PartUsage"))),
        value);
    assertEquals(proto, Protos.proto(value));
  }

  @Test
  void aVerdictValueCarriesItsAssertionAndVerificationKinds() {
    var verdict =
        org.openmbee.opensysml.proto.DocumentVerdict.newBuilder()
            .setAssertion(
                org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                    .setElementId("Garage::Checks::massInRange")
                    .setElementType("ConstraintUsage"))
            .setKind("constraint")
            .setText("assert constraint massInRange")
            .setPath("Garage::car")
            .setVerdict("holds")
            .addVerification("pass")
            .build();
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder().setVerdict(verdict).build();
    DocumentValue value = Protos.documentValue(proto);
    assertInstanceOf(DocumentValue.DocumentVerdict.class, value);
    var read = (DocumentValue.DocumentVerdict) value;
    assertEquals(
        new DocumentValue.ElementRef("Garage::Checks::massInRange", "ConstraintUsage"),
        read.assertion());
    assertEquals("holds", read.status());
    assertEquals(List.of("pass"), read.verification());
    assertEquals(proto, Protos.proto(value));
  }

  @Test
  void aStateValueCarriesItsMachineLeafAndEnclosures() {
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
            .setState(
                org.openmbee.opensysml.proto.DocumentState.newBuilder()
                    .setObject(
                        org.openmbee.opensysml.proto.DocumentObject.newBuilder()
                            .setInstanceId(3)
                            .setPath("Car::lights"))
                    .setMachine("lp")
                    .setName("dim")
                    .setStatePath("on.dim")
                    .setRegion("light")
                    .addEnclosing("on"))
            .build();
    DocumentValue value = Protos.documentValue(proto);
    assertInstanceOf(DocumentValue.DocumentState.class, value);
    var read = (DocumentValue.DocumentState) value;
    assertEquals(3, read.object().id());
    assertEquals("on.dim", read.path());
    assertEquals(List.of("on"), read.enclosing());
    assertEquals(proto, Protos.proto(value));
  }

  @Test
  void anEventValueCarriesItsKindTimeAndRoles() {
    var proto =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
            .setEvent(
                org.openmbee.opensysml.proto.DocumentEvent.newBuilder()
                    .setKind("transition")
                    .setTime(
                        org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                            .setRealValue(1.5))
                    .setText("transition")
                    .setMachine("lp")
                    .setFrom("off")
                    .setTo("on")
                    .setEvent("")
                    .setTaken(""))
            .build();
    DocumentValue value = Protos.documentValue(proto);
    assertInstanceOf(DocumentValue.DocumentEvent.class, value);
    var read = (DocumentValue.DocumentEvent) value;
    assertEquals("transition", read.kind());
    assertEquals(new DocumentValue.RealValue(1.5), read.time());
    assertEquals("off", read.from());
    assertEquals("on", read.to());
    assertEquals(proto, Protos.proto(value));
  }

  private static DocumentRow rowOf(org.openmbee.opensysml.proto.DocumentValue element) {
    var response =
        RunDocumentQueryResponse.newBuilder()
            .addColumns(org.openmbee.opensysml.proto.DocumentQueryColumn.newBuilder().setName("name"))
            .addRows(
                org.openmbee.opensysml.proto.DocumentQueryRow.newBuilder()
                    .setElement(element)
                    .addCells(
                        org.openmbee.opensysml.proto.DocumentQueryCell.newBuilder()
                            .addValues(
                                org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                                    .setStringValue("x"))))
            .build();
    return Protos.documentQueryResult(response).rows().get(0);
  }

  @Test
  void anElementRowCarriesTheElementAndCells() {
    DocumentRow row =
        rowOf(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                .setElementId("Observatory::telescope")
                .setElementType("PartUsage")
                .build());
    assertEquals(new DocumentValue.ElementRef("Observatory::telescope", "PartUsage"), row.element());
    assertEquals(1, row.cells().size());
    assertEquals(List.of(new DocumentValue.StringValue("x")), row.cells().get(0));
    assertTrue(row.verdict().isEmpty());
    assertTrue(row.object().isEmpty());
    assertTrue(row.state().isEmpty());
    assertTrue(row.event().isEmpty());
  }

  @Test
  void aVerdictRowAnswersTheAssertionAsItsElement() {
    DocumentRow row =
        rowOf(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                .setVerdict(
                    org.openmbee.opensysml.proto.DocumentVerdict.newBuilder()
                        .setAssertion(
                            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                                .setElementId("Garage::Checks::massInRange"))
                        .setKind("requirement")
                        .setVerdict("violated")
                        .setReason("mass exceeds"))
                .build());
    assertTrue(row.verdict().isPresent());
    assertEquals("violated", row.verdict().orElseThrow().status());
    assertEquals(new DocumentValue.ElementRef("Garage::Checks::massInRange", ""), row.element());
    assertTrue(row.object().isEmpty());
  }

  @Test
  void anObjectRowAnswersTheHoldingUsageAsItsElement() {
    var objectRef =
        org.openmbee.opensysml.proto.DocumentObject.newBuilder()
            .setInstanceId(4)
            .setPath("Garage::car")
            .setElement(
                org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                    .setElementId("Garage::car")
                    .setElementType("PartUsage"));
    DocumentRow row =
        rowOf(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                .setObject(objectRef)
                .build());
    assertTrue(row.object().isPresent());
    assertEquals(4, row.object().orElseThrow().id());
    assertEquals(new DocumentValue.ElementRef("Garage::car", "PartUsage"), row.element());
  }

  @Test
  void stateAndEventRowsAnswerTheirObjectsElement() {
    DocumentRow stateRow =
        rowOf(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                .setState(
                    org.openmbee.opensysml.proto.DocumentState.newBuilder()
                        .setObject(
                            org.openmbee.opensysml.proto.DocumentObject.newBuilder()
                                .setInstanceId(3)
                                .setPath("Car::lights")
                                .setElement(
                                    org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                                        .setElementId("Car::lights")
                                        .setElementType("PartUsage")))
                        .setName("dim")
                        .setStatePath("on.dim"))
                .build());
    assertTrue(stateRow.state().isPresent());
    assertTrue(stateRow.object().isPresent());
    assertEquals(new DocumentValue.ElementRef("Car::lights", "PartUsage"), stateRow.element());

    var eventObject =
        org.openmbee.opensysml.proto.DocumentObject.newBuilder()
            .setInstanceId(3)
            .setElement(
                org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                    .setElementId("Car::lights"));
    DocumentRow eventRow =
        rowOf(
            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                .setEvent(
                    org.openmbee.opensysml.proto.DocumentEvent.newBuilder()
                        .setKind("accept")
                        .setTime(
                            org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                                .setIntValue(0))
                        .setObject(eventObject))
                .build());
    assertTrue(eventRow.event().isPresent());
    assertTrue(eventRow.object().isPresent());
    assertEquals(new DocumentValue.ElementRef("Car::lights", ""), eventRow.element());
  }
}
