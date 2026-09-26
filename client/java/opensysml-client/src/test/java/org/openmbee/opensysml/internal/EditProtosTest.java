package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Edit;
import org.openmbee.opensysml.EditFailure;
import org.openmbee.opensysml.Language;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.SweepRange;
import org.openmbee.opensysml.Value;

/** Edits, source documents and sweep ranges as the wire carries them, both directions. */
class EditProtosTest {

  @Test
  void aFileDocumentCarriesItsPathAndNoContent() {
    var proto = Protos.proto(SourceDocument.file(Path.of("/tmp/a.sysml")));
    assertEquals("/tmp/a.sysml", proto.getFilePath());
    assertFalse(proto.hasContent());
    assertEquals("", proto.getName());
  }

  @Test
  void anInlineDocumentCarriesItsContentNameAndLanguage() {
    var proto =
        Protos.proto(
            SourceDocument.inline("lib.kerml", "package L {}").withLanguage(Language.KERML));
    assertEquals("lib.kerml", proto.getName());
    assertEquals("package L {}", proto.getContent());
    assertEquals("kerml", proto.getLanguage());
    assertFalse(proto.hasFilePath());
  }

  @Test
  void aSetValueEditCarriesTargetAndValue() {
    var proto = Protos.proto(new Edit.SetValue("Demo::SC::unitMass", "100.0"));
    assertEquals("Demo::SC::unitMass", proto.getSetValue().getTarget());
    assertEquals("100.0", proto.getSetValue().getValue());
    assertEquals(
        org.openmbee.opensysml.proto.EditOperation.OperationCase.SET_VALUE,
        proto.getOperationCase());
  }

  @Test
  void aRenameEditCarriesTargetAndNewName() {
    var proto = Protos.proto(new Edit.Rename("Demo::A", "B"));
    assertEquals("Demo::A", proto.getRename().getTarget());
    assertEquals("B", proto.getRename().getNewName());
  }

  @Test
  void anAddMemberEditCarriesEveryFieldItNames() {
    var minimal = Protos.proto(Edit.AddMember.of("Demo::A", "part", "b"));
    assertEquals("Demo::A", minimal.getAddMember().getOwner());
    assertEquals("part", minimal.getAddMember().getKind());
    assertEquals("b", minimal.getAddMember().getName());
    assertEquals("", minimal.getAddMember().getType());
    assertEquals(0, minimal.getAddMember().getSpecializesCount());

    var full =
        Protos.proto(
            Edit.AddMember.of("Demo::A", "attribute", "x")
                .withType("Real")
                .withMultiplicity("0..1")
                .withValue("1.0")
                .withSpecializes(List.of("Demo::A::y"))
                .withAbstract(true)
                .withRedefines(List.of("Demo::A::old"))
                .withDefault(true)
                .withDirection("in"));
    assertEquals("Real", full.getAddMember().getType());
    assertEquals("0..1", full.getAddMember().getMultiplicity());
    assertEquals("1.0", full.getAddMember().getValue());
    assertEquals(List.of("Demo::A::y"), full.getAddMember().getSpecializesList());
    assertTrue(full.getAddMember().getIsAbstract());
    assertEquals(List.of("Demo::A::old"), full.getAddMember().getRedefinesList());
    assertTrue(full.getAddMember().getIsDefault());
    assertEquals("in", full.getAddMember().getDirection());
  }

  @Test
  void anAddSatisfyEditCarriesItsRequirementAndFlags() {
    var operation =
        Protos.proto(
            Edit.AddSatisfy.of("Demo::r", "Demo::r")
                .withSatisfyingFeature("Demo::t")
                .withAsserted(true)
                .withNegated(true));
    assertEquals("Demo::r", operation.getAddSatisfy().getOwner());
    assertEquals("Demo::r", operation.getAddSatisfy().getRequirement());
    assertEquals("Demo::t", operation.getAddSatisfy().getSatisfyingFeature());
    assertTrue(operation.getAddSatisfy().getIsAsserted());
    assertTrue(operation.getAddSatisfy().getIsNegated());
  }

  @Test
  void anAddRequirementConstraintEditCarriesItsExpressionAndName() {
    var operation =
        Protos.proto(
            Edit.AddRequirementConstraint.of("Demo::r", "require", "true")
                .withName("valid"));
    assertEquals("Demo::r", operation.getAddRequirementConstraint().getOwner());
    assertEquals("require", operation.getAddRequirementConstraint().getKind());
    assertEquals("true", operation.getAddRequirementConstraint().getExpression());
    assertEquals("valid", operation.getAddRequirementConstraint().getName());
  }

  @Test
  void anAddConnectionEditCarriesItsEndsAndOptionalFields() {
    var minimal = Protos.proto(Edit.AddConnection.of("Demo::System", "flow", "a.out", "b.in"));
    assertEquals("Demo::System", minimal.getAddConnection().getOwner());
    assertEquals("flow", minimal.getAddConnection().getKind());
    assertEquals("a.out", minimal.getAddConnection().getFromEnd());
    assertEquals("b.in", minimal.getAddConnection().getToEnd());
    assertEquals("", minimal.getAddConnection().getName());
    assertEquals("", minimal.getAddConnection().getType());

    var full =
        Protos.proto(
            Edit.AddConnection.of("Demo::System", "allocation", "a", "b")
                .withName("alloc1")
                .withType("AllocationType"));
    assertEquals("alloc1", full.getAddConnection().getName());
    assertEquals("AllocationType", full.getAddConnection().getType());
  }

  @Test
  void aDeleteEditCarriesItsCascadeAndAMoveEditItsOwner() {
    var delete = Protos.proto(new Edit.Delete("Demo::A", true));
    assertEquals("Demo::A", delete.getDelete().getTarget());
    assertTrue(delete.getDelete().getCascade());
    var move = Protos.proto(new Edit.Move("Demo::A::b", "Demo::C"));
    assertEquals("Demo::A::b", move.getMove().getTarget());
    assertEquals("Demo::C", move.getMove().getOwner());
  }

  @Test
  void everyKnownEditFailureReadsAsItselfAndANewOneAsUnrecognized() {
    assertEquals(
        EditFailure.UNKNOWN_TARGET,
        Protos.editFailure(org.openmbee.opensysml.proto.EditFailure.EDIT_FAILURE_UNKNOWN_TARGET));
    assertEquals(
        EditFailure.REFERENCED_ELSEWHERE,
        Protos.editFailure(
            org.openmbee.opensysml.proto.EditFailure.EDIT_FAILURE_REFERENCED_ELSEWHERE));
    var future = org.openmbee.opensysml.proto.EditFailure.UNRECOGNIZED;
    assertEquals(EditFailure.UNRECOGNIZED, Protos.editFailure(future));
    assertEquals("EDIT_FAILURE_99", Protos.editFailureName(future, 99));
    assertEquals(
        "EDIT_FAILURE_UNKNOWN_TARGET",
        Protos.editFailureName(
            org.openmbee.opensysml.proto.EditFailure.EDIT_FAILURE_UNKNOWN_TARGET,
            org.openmbee.opensysml.proto.EditFailure.EDIT_FAILURE_UNKNOWN_TARGET.getNumber()));
  }

  @Test
  void aSweepRangeCarriesItsStepOnlyWhenNamed() {
    var stepped =
        Protos.proto(
            SweepRange.of("b", new Value.IntegerValue(1), new Value.IntegerValue(4))
                .withStep(new Value.IntegerValue(1)));
    assertEquals("b", stepped.getParameter());
    assertEquals(1, stepped.getStart().getIntValue());
    assertEquals(4, stepped.getEnd().getIntValue());
    assertEquals(1, stepped.getStep().getIntValue());

    var set =
        Protos.proto(
            SweepRange.of("b", new Value.IntegerValue(1), new Value.IntegerValue(4)));
    assertFalse(set.hasStep());
  }
}
