package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.openmbee.opensysml.internal.ConnectTransport;

/** The authoring builder and the client-side helpers that need no service. */
class EditorTest {

  private static Connection offline(String... capabilities) {
    return new Connection(
        new ConnectTransport("127.0.0.1:1", Encoding.PROTOBUF, Duration.ofSeconds(1)),
        new Capabilities("dev", Set.of(capabilities)));
  }

  private static Model model(Connection connection) {
    return new Model(connection, "hash", List.of(), List.of());
  }

  @Test
  void anEditorCollectsTheEditsItsShorthandsName() {
    try (Connection connection = offline()) {
      Editor editor =
          model(connection)
              .edit()
              .setValue("Demo::sc::unitMass", "1050.0[SI::kg]")
              .rename("Demo::Old", "New")
              .addPart("Demo::Vehicle", "engine", m -> m.withType("Engine"))
              .addRequireConstraint("Demo::R", "mass < 2000")
              .addAllocation("Demo::S", "a", "b")
              .delete("Demo::gone", true)
              .move("Demo::x", "Demo::Other");
      List<Edit> edits = editor.edits();
      assertEquals(new Edit.SetValue("Demo::sc::unitMass", "1050.0[SI::kg]"), edits.get(0));
      assertEquals(new Edit.Rename("Demo::Old", "New"), edits.get(1));
      assertEquals(
          Edit.AddMember.of("Demo::Vehicle", "part", "engine").withType("Engine"), edits.get(2));
      assertEquals(
          Edit.AddRequirementConstraint.of("Demo::R", "require", "mass < 2000"), edits.get(3));
      assertEquals(Edit.AddConnection.of("Demo::S", "allocation", "a", "b"), edits.get(4));
      assertEquals(new Edit.Delete("Demo::gone", true), edits.get(5));
      assertEquals(new Edit.Move("Demo::x", "Demo::Other"), edits.get(6));
      assertFalse(editor.applied());
    }
  }

  @Test
  void parameterShorthandsDeclareTheMemberAndItsParameters() {
    try (Connection connection = offline()) {
      List<Edit> edits =
          model(connection)
              .edit()
              .addCalcDef(
                  "Demo", "Double", List.of(new Editor.Parameter("x", "Real")), "Real", "x * 2")
              .addActionDef(
                  "",
                  "Drive",
                  List.of(new Editor.Parameter("speed", "")),
                  List.of(new Editor.Parameter("distance", "Real")))
              .edits();
      assertEquals(
          List.of(
              Edit.AddMember.of("Demo", "calc def", "Double"),
              Edit.AddMember.of("Demo::Double", "", "x").withDirection("in").withType("Real"),
              Edit.AddMember.of("Demo::Double", "return", "").withType("Real").withValue("x * 2"),
              Edit.AddMember.of("", "action def", "Drive"),
              Edit.AddMember.of("Drive", "", "speed").withDirection("in"),
              Edit.AddMember.of("Drive", "", "distance").withDirection("out").withType("Real")),
          edits);
    }
  }

  @Test
  void metadataValuesKeepTheirOrderAndStateActionsAreChecked() {
    try (Connection connection = offline()) {
      Map<String, String> values = new LinkedHashMap<>();
      values.put("level", "2");
      values.put("owner", "\"ops\"");
      Editor editor = model(connection).edit().addMetadata("Demo::P", "Review", values);
      assertEquals(
          Edit.AddMetadata.of("Demo::P", "Review")
              .withValues(
                  List.of(
                      new Edit.MetadataValue("level", "2"),
                      new Edit.MetadataValue("owner", "\"ops\""))),
          editor.edits().get(0));
      assertThrows(
          IllegalArgumentException.class, () -> editor.addStateAction("Demo::S", "during", "x"));
      editor.addStateAction("Demo::S", "entry", "init");
      assertEquals(Edit.AddMember.of("Demo::S", "entry action", "init"), editor.edits().get(1));
    }
  }

  @Test
  void aBodyWritesItsFirstStatementPlainAndLaterOnesWithThen() {
    Editor.Body body = new Editor.Body().addAssign("x", "x + 1").addSend("x", "self");
    Editor.Body elseBody = new Editor.Body().addTerminate();
    try (Connection connection = offline()) {
      Edit.AddSequence conditional =
          (Edit.AddSequence)
              model(connection).edit().addIf("Demo::A", "x < 3", body, elseBody).edits().get(0);
      assertEquals("then", conditional.keyword());
      assertEquals(Optional.of("if"), conditional.memberKind());
      assertEquals(Optional.of("x < 3"), conditional.condition());
      List<Edit.AddSequence> items = conditional.body();
      assertEquals("", items.get(0).keyword());
      assertEquals(Optional.of("assign"), items.get(0).memberKind());
      assertEquals(Optional.of("x"), items.get(0).target());
      assertEquals(Optional.of("x + 1"), items.get(0).value());
      assertEquals("then", items.get(1).keyword());
      assertEquals(Optional.of("send"), items.get(1).memberKind());
      assertEquals(Optional.of("self"), items.get(1).target());
      assertEquals("", conditional.elseBody().get(0).keyword());
      assertEquals(Optional.of("terminate"), conditional.elseBody().get(0).memberKind());
    }
  }

  @Test
  void statementFactoriesCarryEveryActionBodyField() {
    Edit.AddSequence accept =
        Edit.AddSequence.accept("", "message").withType("Signal").withVia("inPort").withoutThen();
    assertEquals("", accept.keyword());
    assertEquals(Optional.of("message"), accept.parameter());
    assertEquals(Optional.of("Signal"), accept.type());
    assertEquals(Optional.of("inPort"), accept.via());
    Edit.AddSequence iteration =
        Edit.AddSequence.forLoop("", "i", "(1, 2)", List.of()).withType("Integer");
    assertEquals(Optional.of("i"), iteration.parameter());
    assertEquals(Optional.of("(1, 2)"), iteration.value());
    Edit.AddSequence guarded = Edit.AddSequence.guardedThen("Demo::A", "x > 0", "done");
    assertEquals("if", guarded.keyword());
    assertEquals(Optional.of("done"), guarded.ref());
    assertEquals(Optional.of("x > 0"), guarded.condition());
    assertEquals("else", Edit.AddSequence.elseThen("Demo::A", "other").keyword());
    Edit.AddSequence counted = Edit.AddSequence.terminate("").withMultiplicity("[1]");
    assertThrows(IllegalStateException.class, counted::withoutThen);
    assertThrows(IllegalStateException.class, guarded::withoutThen);
  }

  @Test
  void anEditorRefusesWhatItsCapabilitiesLackBeforeAnyCall() {
    try (Connection connection = offline(Capabilities.APPLY_EDITS, Capabilities.AUTHORING)) {
      Editor editor = model(connection).edit().addAccept("Demo::A", "message");
      CapabilityException refused = assertThrows(CapabilityException.class, editor::apply);
      assertEquals(Capabilities.SEQUENCE_AUTHORING, refused.capability());
      assertFalse(editor.applied());
    }
  }

  @Test
  void aCapabilityRefusalNamesTheServiceAndTheRemedy() {
    Capabilities capabilities = new Capabilities("v0.9.0", Set.of(Capabilities.RENDER_DOCUMENT));
    CapabilityException refused =
        assertThrows(
            CapabilityException.class,
            () -> capabilities.require(Capabilities.RENDER_DOCUMENT_HTML));
    assertEquals(Capabilities.RENDER_DOCUMENT_HTML, refused.capability());
    assertEquals(Capabilities.upgradeRemedy(Capabilities.RENDER_DOCUMENT_HTML), refused.remedy());
    assertTrue(refused.getMessage().contains("service: sysml-grpc v0.9.0"), refused.getMessage());
    assertTrue(refused.getMessage().contains("make build-grpc"), refused.getMessage());
  }

  @Test
  void htmlRenderingNeedsItsOwnCapability() {
    try (Connection connection = offline(Capabilities.RENDER_DOCUMENT)) {
      Model model = model(connection);
      CapabilityException refused =
          assertThrows(
              CapabilityException.class,
              () -> model.renderDocument("Demo::Doc", DocumentForm.HTML));
      assertEquals(Capabilities.RENDER_DOCUMENT_HTML, refused.capability());
    }
  }

  @Test
  void documentFormsReadTheirWireNames() {
    assertEquals(DocumentForm.MARKDOWN, DocumentForm.fromWireName(""));
    assertEquals(DocumentForm.MARKDOWN, DocumentForm.fromWireName("markdown"));
    assertEquals(DocumentForm.HTML, DocumentForm.fromWireName("html"));
    assertThrows(IllegalArgumentException.class, () -> DocumentForm.fromWireName("pdf"));
    RenderedDocument html = new RenderedDocument(DocumentForm.HTML, "<p>x</p>");
    assertEquals("<p>x</p>", html.html());
    assertEquals("", html.markdown());
    assertEquals("# x", new RenderedDocument("# x").markdown());
  }

  @Test
  void conversionOptionsCarryTheIdForm() {
    ConversionOptions options =
        ConversionOptions.defaults()
            .withIdForm(ConversionOptions.ID_FORM_UUID)
            .withFromFormat("sysml")
            .withTolerateSyntaxErrors(true);
    assertEquals(Optional.of("uuid"), options.idForm());
    assertEquals(Optional.of("sysml"), options.fromFormat());
    assertEquals(Optional.empty(), ConversionOptions.defaults().idForm());
  }

  @Test
  void aConversionNamesTheFormatOfAPathAndWritesItsContent(@TempDir Path directory)
      throws Exception {
    assertEquals("sysml", Conversion.formatOf(Path.of("a.sysml")));
    assertEquals("ttl", Conversion.formatOf(Path.of("a.ttl")));
    assertEquals("api-json", Conversion.formatOf(Path.of("a.json")));
    assertThrows(IllegalArgumentException.class, () -> Conversion.formatOf(Path.of("a.xmi")));
    Path written = Conversion.writeContent("package P;\r\n", directory.resolve("out.sysml"));
    assertEquals("package P;\r\n", Files.readString(written));
  }

  @Test
  void aModelWithErrorsIsNotOkAndRefusesToPassAsClean() {
    try (Connection connection = offline()) {
      Diagnostic error = new Diagnostic(Diagnostic.Severity.ERROR, "expected ';'", "", Optional.empty());
      Diagnostic warning = new Diagnostic(Diagnostic.Severity.WARNING, "unused", "", Optional.empty());
      Model clean = new Model(connection, "hash", List.of(), List.of(warning));
      assertTrue(clean.ok());
      assertEquals(clean, clean.requireNoErrors());
      Model broken =
          new Model(
              connection, "hash", List.of(), List.of(error, error, error, error, error));
      assertFalse(broken.ok());
      ModelException refused = assertThrows(ModelException.class, broken::requireNoErrors);
      assertTrue(refused.getMessage().contains("5 error(s)"), refused.getMessage());
      assertTrue(refused.getMessage().contains("and 2 more"), refused.getMessage());
      assertEquals(5, refused.diagnostics().size());
    }
  }

  @Test
  void aShortNameOnAnAdoptedModelNeedsQueryRatherThanAnsweringAbsent() {
    try (Connection connection = offline()) {
      CapabilityException refused =
          assertThrows(CapabilityException.class, () -> model(connection).find("Vehicle"));
      assertEquals(Capabilities.QUERY, refused.capability());
    }
  }

  @Test
  void anEditOfSeveralDocumentsIsNotSavedAsOneFile(@TempDir Path directory) throws Exception {
    EditResult several =
        new EditResult(
            "",
            List.of(),
            List.of(new EditedDocument("a.sysml", "package A;"), new EditedDocument("b.sysml", "")),
            List.of());
    Path target = directory.resolve("out.sysml");
    Files.writeString(target, "kept");
    assertThrows(IllegalStateException.class, () -> several.save(target));
    assertEquals("kept", Files.readString(target));
    EditResult oneOfSeveral =
        new EditResult(
            "", List.of(), List.of(new EditedDocument("a.sysml", "package A;")), List.of());
    assertThrows(IllegalStateException.class, () -> oneOfSeveral.save(target));
    assertEquals("kept", Files.readString(target));
    EditResult one = new EditResult("package A;", List.of(), List.of(), List.of());
    assertEquals("package A;", Files.readString(one.save(target)));
    EditResult emptied =
        new EditResult("", List.of(), List.of(new EditedDocument("a.sysml", "")), List.of());
    assertEquals("", Files.readString(emptied.save(target)));
    EditResult emptiedOfSeveral =
        new EditResult(
            "", List.of(), List.of(new EditedDocument("a.sysml", "")), List.of(), true);
    Files.writeString(target, "kept");
    assertThrows(IllegalStateException.class, () -> emptiedOfSeveral.save(target));
    assertEquals("kept", Files.readString(target));
  }

  @Test
  void aMissingSymbolSuggestsNearNames() {
    SymbolNotFoundException missing =
        new SymbolNotFoundException("Vehicel", NearNames.closest(
            "Vehicel", List.of("Vehicle", "Engine", "Demo::Vehicle"), 3));
    assertEquals("Vehicel", missing.name());
    assertEquals(List.of("Vehicle", "Demo::Vehicle"), missing.suggestions());
    assertTrue(missing.getMessage().contains("'Vehicel'"), missing.getMessage());
    assertTrue(missing.getMessage().contains("did you mean 'Vehicle'"), missing.getMessage());
    assertInstanceOf(ModelException.class, missing);
    assertEquals(0.75, NearNames.ratio("abcd", "bcde"), 1e-9);
  }
}
