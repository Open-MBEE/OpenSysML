package org.openmbee.mdk.magic.engine;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.opensysml.DocumentForm;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Symbol;

/** Renders the documents and result views a subject stands for, as Markdown and HTML files named by what was rendered. */
final class DocumentRenderer {
  static final String DOCUMENT = "DocumentQueries::Document";
  static final String RESULT_VIEW = "QVTS::QueryResult";
  private static final List<DocumentForm> FORMS = List.of(DocumentForm.MARKDOWN, DocumentForm.HTML);

  private DocumentRenderer() {}

  static List<Path> render(ModelSource source, Model model, String subjectQualifiedName, Path directory) {
    String subject = source.subjectName(subjectQualifiedName);
    Set<String> candidates = new LinkedHashSet<>();
    candidates.add(subject);
    for (String target : source.mappedTargets(subjectQualifiedName)) candidates.add(QueryResolver.targetName(target));
    Map<String, String> rendered = new LinkedHashMap<>();
    for (String candidate : candidates) {
      Optional<Symbol> symbol = model.findSymbol(candidate);
      if (symbol.isEmpty()) continue;
      boolean document = specializes(model, symbol.get(), DOCUMENT, new HashSet<>());
      if (!document && !specializes(model, symbol.get(), RESULT_VIEW, new HashSet<>())) continue;
      for (DocumentForm form : FORMS) {
        String content = document
            ? content(form, model.renderDocument(symbol.get().id(), form))
            : model.renderView(symbol.get().id(), form).document();
        rendered.put(fileName(symbol.get().id(), form), content);
      }
    }
    if (rendered.isEmpty()) {
      throw new IllegalArgumentException(
          subject + " is neither a " + DOCUMENT + " nor a " + RESULT_VIEW + " view"
              + (candidates.size() > 1 ? ", and the migration maps it to none" : ""));
    }
    List<Path> written = new ArrayList<>();
    try {
      Files.createDirectories(directory);
      for (Map.Entry<String, String> file : rendered.entrySet()) {
        Path target = directory.resolve(file.getKey());
        Files.writeString(target, file.getValue(), StandardCharsets.UTF_8);
        written.add(target);
      }
    } catch (IOException exception) {
      throw new UncheckedIOException("cannot write to " + directory + ": " + exception.getMessage(), exception);
    }
    return List.copyOf(written);
  }

  private static String content(DocumentForm form, org.openmbee.opensysml.RenderedDocument document) {
    return form == DocumentForm.HTML ? document.html() : document.markdown();
  }

  static String fileName(String qualifiedName, DocumentForm form) {
    String stem = qualifiedName.replace("::", ".").replaceAll("[^A-Za-z0-9._-]", "_");
    return stem + (form == DocumentForm.HTML ? ".html" : ".md");
  }

  /** Whether {@code symbol} is typed by or specializes {@code definition}, directly or through its generals. */
  private static boolean specializes(Model model, Symbol symbol, String definition, Set<String> visited) {
    if (!visited.add(symbol.id())) return false;
    if (symbol.id().equals(definition)) return true;
    for (Symbol.Specialization general : symbol.specializations()) {
      if (general.targetId().isEmpty()) continue;
      if (general.targetId().get().equals(definition)) return true;
      Optional<Symbol> target = model.findSymbol(general.targetId().get());
      if (target.isPresent() && specializes(model, target.get(), definition, visited)) return true;
    }
    return false;
  }
}
