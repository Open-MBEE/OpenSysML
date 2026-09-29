package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
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
                .withDirection("in")
                .withBodyExpression("x > 1")
                .withDoc("A value."));
    assertEquals("Real", full.getAddMember().getType());
    assertEquals("0..1", full.getAddMember().getMultiplicity());
    assertEquals("1.0", full.getAddMember().getValue());
    assertEquals(List.of("Demo::A::y"), full.getAddMember().getSpecializesList());
    assertTrue(full.getAddMember().getIsAbstract());
    assertEquals(List.of("Demo::A::old"), full.getAddMember().getRedefinesList());
    assertTrue(full.getAddMember().getIsDefault());
    assertEquals("in", full.getAddMember().getDirection());
    assertEquals("x > 1", full.getAddMember().getBodyExpression());
    assertEquals("A value.", full.getAddMember().getDoc());
    assertEquals("", minimal.getAddMember().getDoc());
  }

  @Test
  void anAddDocumentationEditCarriesItsBodyNameLocaleAndReplace() {
    var minimal = Protos.proto(Edit.AddDocumentation.of("Demo::A", "Text."));
    assertEquals("Demo::A", minimal.getAddDocumentation().getTarget());
    assertEquals("Text.", minimal.getAddDocumentation().getBody());
    assertEquals("", minimal.getAddDocumentation().getName());
    assertFalse(minimal.getAddDocumentation().getReplace());

    var full =
        Protos.proto(
            Edit.AddDocumentation.of("Demo::A", "Text.")
                .withName("Summary")
                .withLocale("en")
                .withReplace(true));
    assertEquals("Summary", full.getAddDocumentation().getName());
    assertEquals("en", full.getAddDocumentation().getLocale());
    assertTrue(full.getAddDocumentation().getReplace());
  }

  @Test
  void anAddCommentEditCarriesItsBodyNameAboutAndLocale() {
    var minimal = Protos.proto(Edit.AddComment.of("", "Text."));
    assertEquals("", minimal.getAddComment().getOwner());
    assertEquals("Text.", minimal.getAddComment().getBody());
    assertEquals(0, minimal.getAddComment().getAboutCount());

    var full =
        Protos.proto(
            Edit.AddComment.of("Demo", " Two\nlines ")
                .withName("Why")
                .withAbout(java.util.List.of("Demo::A", "Demo"))
                .withLocale("en"));
    assertEquals(" Two\nlines ", full.getAddComment().getBody());
    assertEquals("Why", full.getAddComment().getName());
    assertEquals(java.util.List.of("Demo::A", "Demo"), full.getAddComment().getAboutList());
    assertEquals("en", full.getAddComment().getLocale());
  }

  @Test
  void anAddNoteEditCarriesItsTargetAndOneLineOfText() {
    var operation = Protos.proto(new Edit.AddNote("Demo::A", "DimensionOneValue"));
    assertEquals("Demo::A", operation.getAddNote().getTarget());
    assertEquals("DimensionOneValue", operation.getAddNote().getText());
    org.junit.jupiter.api.Assertions.assertThrows(
        IllegalArgumentException.class, () -> new Edit.AddNote("Demo::A", "one\ntwo"));
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
  void anAddTransitionEditCarriesClausesAndEntryFlag() {
    var operation =
        Protos.proto(
            Edit.AddTransition.of("Demo::S", "idle", "toasting")
                .withName("go")
                .withTrigger("CycleStart")
                .withGuard("ready")
                .withEffect("action cool"));
    assertEquals("Demo::S", operation.getAddTransition().getOwner());
    assertEquals("go", operation.getAddTransition().getName());
    assertEquals("idle", operation.getAddTransition().getSource());
    assertEquals("toasting", operation.getAddTransition().getTarget());
    assertEquals("CycleStart", operation.getAddTransition().getTrigger());
    assertEquals("ready", operation.getAddTransition().getGuard());
    assertEquals("action cool", operation.getAddTransition().getEffect());
    assertFalse(operation.getAddTransition().getInitial());

    var entry = Protos.proto(Edit.AddTransition.entry("Demo::S", "idle"));
    assertEquals("idle", entry.getAddTransition().getTarget());
    assertTrue(entry.getAddTransition().getInitial());
  }

  @Test
  void anAddSequenceEditCarriesItsKeywordAndEnds() {
    var operation =
        Protos.proto(
            Edit.AddSequence.thenMember("Demo::A", "action", "b")
                .withType("B")
                .withAfter("a"));
    assertEquals("Demo::A", operation.getAddSequence().getOwner());
    assertEquals("then", operation.getAddSequence().getKeyword());
    assertEquals("action", operation.getAddSequence().getMemberKind());
    assertEquals("b", operation.getAddSequence().getMemberName());
    assertEquals("B", operation.getAddSequence().getType());
    assertEquals("a", operation.getAddSequence().getAfter());

    var first = Protos.proto(Edit.AddSequence.first("Demo::A", "start"));
    assertEquals("first", first.getAddSequence().getKeyword());
    assertEquals("start", first.getAddSequence().getRef());

    assertThrows(
        IllegalStateException.class,
        () -> Edit.AddSequence.then("Demo::A", "done").withType("B"));
    assertThrows(
        IllegalStateException.class,
        () -> Edit.AddSequence.thenMember("Demo::A", "action", "b").withRef("done"));
  }

  @Test
  void anAcceptTypeCanBeSetBeforeOrAfterItsParameter() {
    var afterParameter =
        Protos.proto(
            Edit.AddSequence.thenMember("Demo::A", "accept", "")
                .withParameter("payload")
                .withType("Signal"));
    var beforeParameter =
        Protos.proto(
            Edit.AddSequence.thenMember("Demo::A", "accept", "")
                .withType("Signal")
                .withParameter("payload"));

    for (var operation : List.of(afterParameter, beforeParameter)) {
      assertEquals("accept", operation.getAddSequence().getMemberKind());
      assertEquals("Signal", operation.getAddSequence().getType());
      assertEquals("payload", operation.getAddSequence().getParameter());
    }
  }

  @Test
  void aForTypeCanBeSetBeforeOrAfterItsParameterAndBody() {
    var body =
        List.of(
            Edit.AddSequence.thenMember("", "assign", "result")
                .withTarget("result")
                .withValue("i"));
    var afterFields =
        Protos.proto(
            Edit.AddSequence.thenMember("Demo::A", "for", "")
                .withParameter("i")
                .withValue("items")
                .withBody(body)
                .withType("Integer"));
    var beforeFields =
        Protos.proto(
            Edit.AddSequence.thenMember("Demo::A", "for", "")
                .withType("Integer")
                .withParameter("i")
                .withValue("items")
                .withBody(body));

    for (var operation : List.of(afterFields, beforeFields)) {
      assertEquals("for", operation.getAddSequence().getMemberKind());
      assertEquals("Integer", operation.getAddSequence().getType());
      assertEquals("i", operation.getAddSequence().getParameter());
      assertEquals("items", operation.getAddSequence().getValue());
      assertEquals(1, operation.getAddSequence().getBodyCount());
    }
  }

  @Test
  void anAddSequenceEditCarriesRecursiveActionBodyFields() {
    var nested =
        new Edit.AddSequence(
            "", "", java.util.Optional.empty(), java.util.Optional.of("assign"),
            java.util.Optional.empty(), java.util.Optional.empty(), java.util.Optional.empty(),
            java.util.Optional.empty(), java.util.Optional.of("x + 1"),
            java.util.Optional.of("x"), java.util.Optional.empty(), java.util.Optional.empty(),
            List.of(), List.of(), java.util.Optional.empty(), java.util.Optional.empty());
    var inner =
        new Edit.AddSequence(
            "", "", java.util.Optional.empty(), java.util.Optional.of("if"),
            java.util.Optional.empty(), java.util.Optional.empty(), java.util.Optional.empty(),
            java.util.Optional.of("ready"), java.util.Optional.empty(), java.util.Optional.empty(),
            java.util.Optional.empty(), java.util.Optional.empty(), List.of(nested), List.of(),
            java.util.Optional.empty(), java.util.Optional.empty());
    var root =
        new Edit.AddSequence(
            "Demo::A", "then", java.util.Optional.empty(), java.util.Optional.of("if"),
            java.util.Optional.empty(), java.util.Optional.empty(), java.util.Optional.empty(),
            java.util.Optional.of("ready"), java.util.Optional.empty(), java.util.Optional.empty(),
            java.util.Optional.empty(), java.util.Optional.empty(), List.of(inner), List.of(),
            java.util.Optional.of("[1]"), java.util.Optional.empty());

    var operation = Protos.proto(root).getAddSequence();
    assertEquals("[1]", operation.getMultiplicity());
    assertEquals("ready", operation.getCondition());
    assertEquals("if", operation.getBody(0).getMemberKind());
    assertEquals("assign", operation.getBody(0).getBody(0).getMemberKind());
    assertEquals("x + 1", operation.getBody(0).getBody(0).getValue());
  }

  @Test
  void anExtendedAddSequenceEditCarriesItsRecursiveActionBodyFields() throws Exception {
    var nested =
        org.openmbee.opensysml.proto.AddSequenceEdit.newBuilder()
            .setMemberKind("assign")
            .setTarget("x")
            .setValue("x + 1")
            .build();
    var edit =
        org.openmbee.opensysml.proto.AddSequenceEdit.newBuilder()
            .setOwner("Demo::A")
            .setKeyword("then")
            .setMemberKind("if")
            .setCondition("ready")
            .setMultiplicity("[1]")
            .addBody(nested)
            .addElseBody(
                org.openmbee.opensysml.proto.AddSequenceEdit.newBuilder()
                    .setKeyword("else")
                    .setRef("done"))
            .build();
    var decoded = org.openmbee.opensysml.proto.AddSequenceEdit.parseFrom(edit.toByteArray());
    assertEquals(edit, decoded);
    assertEquals("if", decoded.getMemberKind());
    assertEquals("ready", decoded.getCondition());
    assertEquals("[1]", decoded.getMultiplicity());
    assertEquals("x", decoded.getBody(0).getTarget());
    assertEquals("x + 1", decoded.getBody(0).getValue());
    assertEquals("done", decoded.getElseBody(0).getRef());
  }

  @Test
  void anAddImportEditCarriesItsFlagsAndFilters() {
    var operation =
        Protos.proto(
            Edit.AddImport.of("Demo", "ScalarValues::*")
                .withVisibility("public")
                .withRecursive()
                .withAll()
                .withFilters(List.of("@Safety", "@Approved")));
    assertEquals("Demo", operation.getAddImport().getOwner());
    assertEquals("public", operation.getAddImport().getVisibility());
    assertEquals("ScalarValues::*", operation.getAddImport().getTarget());
    assertTrue(operation.getAddImport().getIsRecursive());
    assertTrue(operation.getAddImport().getIsImportAll());
    assertEquals(List.of("@Safety", "@Approved"), operation.getAddImport().getFiltersList());
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
