package org.openmbee.mdk.magic.engine;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import org.openmbee.mdk.magic.identity.QualifiedNames;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.opensysml.ElementProperties;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Symbol;

/**
 * Finds the {@code DocumentQueries::Query} definitions a subject stands for: the subject itself
 * when it is one, else — on a migrated project — the queries behind what the migration wrote for
 * it: a Document, a section or a table whose rows a {@code calc} member computes.
 */
public final class QueryResolver {
  static final String QUERY = "DocumentQueries::Query";
  private static final int MEMBER_DEPTH = 12;

  private QueryResolver() {}

  static QueryPlan plan(RunRequest request, Model model) {
    ModelSource source = request.source();
    String subject = source.subjectName(request.subjectQualifiedName());
    List<String> queries = queries(source, model, request.subjectQualifiedName());
    if (queries.isEmpty()) {
      String why = source.path() == ModelPath.V1_MDZIP
          ? " and the migration maps it to no Document, section or table over one"
          : "";
      throw new IllegalArgumentException(
          subject + " is not a calc def specializing " + QUERY + why);
    }
    List<QueryParameter> parameters = new ArrayList<>();
    for (String query : queries) parameters.addAll(parameters(model, query));
    return new QueryPlan(queries, parameters);
  }

  /** The queries, in order of discovery, without repeats. */
  static List<String> queries(ModelSource source, Model model, String subjectQualifiedName) {
    Set<String> found = new LinkedHashSet<>();
    model.findSymbol(source.subjectName(subjectQualifiedName))
        .ifPresent(symbol -> collect(model, symbol, new HashSet<>(), found, 0));
    for (String target : source.mappedTargets(subjectQualifiedName)) {
      model.findSymbol(targetName(target))
          .ifPresent(symbol -> collect(model, symbol, new HashSet<>(), found, 0));
    }
    return List.copyOf(found);
  }

  /** A migration report target is the declaration's keywords, then its qualified name. */
  public static String targetName(String target) {
    String rest = target.strip();
    while (true) {
      int space = rest.indexOf(' ');
      if (space <= 0 || !rest.substring(0, space).chars().allMatch(Character::isLowerCase)) break;
      rest = rest.substring(space + 1).strip();
    }
    return QualifiedNames.normalize(rest);
  }

  private static void collect(Model model, Symbol symbol, Set<String> visited, Set<String> found, int depth) {
    if (!visited.add(symbol.id()) || depth > MEMBER_DEPTH) return;
    if (isQuery(model, symbol)) {
      found.add(symbol.id());
      return;
    }
    if (!symbol.kind().endsWith("Usage") && !symbol.kind().endsWith("Def")) return;
    if (symbol.kind().equals("calcUsage")) {
      typeOf(symbol).flatMap(model::findSymbol)
          .filter(type -> isQuery(model, type))
          .ifPresent(type -> found.add(type.id()));
      return;
    }
    for (Symbol.Member member : symbol.members()) {
      if (member.library() || !member.kind().endsWith("Usage")) continue;
      model.findSymbol(member.id()).ifPresent(child -> collect(model, child, visited, found, depth + 1));
    }
  }

  private static Optional<String> typeOf(Symbol symbol) {
    Optional<String> resolved = symbol.typeFacts().flatMap(Symbol.TypeFacts::resolvedId).filter(id -> !id.isBlank());
    if (resolved.isPresent()) return resolved;
    return symbol.specializations().stream()
        .filter(item -> item.kind().equals("typing"))
        .flatMap(item -> item.targetId().stream())
        .findFirst();
  }

  /** A calc def specializing {@code DocumentQueries::Query}, directly or through other calc defs. */
  static boolean isQuery(Model model, Symbol symbol) {
    return isQuery(model, symbol, new HashSet<>());
  }

  private static boolean isQuery(Model model, Symbol symbol, Set<String> visited) {
    if (!symbol.kind().equals("calcDef") || !visited.add(symbol.id())) return false;
    if (symbol.id().equals(QUERY)) return true;
    for (Symbol.Specialization general : symbol.specializations()) {
      if (general.targetId().isEmpty() || general.kind().equals("typing")) continue;
      if (general.targetId().get().equals(QUERY)) return true;
      Optional<Symbol> target = model.findSymbol(general.targetId().get());
      if (target.isPresent() && isQuery(model, target.get(), visited)) return true;
    }
    return false;
  }

  /**
   * The {@code in} parameters of {@code query}, declared or inherited, in declaration order; a
   * redefinition stands for the parameter it redefines.
   */
  static List<QueryParameter> parameters(Model model, String query) {
    List<QueryParameter> parameters = new ArrayList<>();
    Optional<Symbol> definition = model.findSymbol(query);
    if (definition.isEmpty()) return parameters;
    List<Symbol.Member> members = new ArrayList<>(definition.get().members());
    members.sort((a, b) -> Boolean.compare(a.inherited(), b.inherited()));
    Set<String> named = new HashSet<>();
    for (Symbol.Member member : members) {
      if (member.library() || !named.add(member.name())) continue;
      Optional<Symbol> symbol = model.findSymbol(member.id());
      if (symbol.isEmpty() || !"in".equals(symbol.get().metadata().get("direction"))) continue;
      parameters.add(parameter(model, query, symbol.get()));
    }
    return parameters;
  }

  private static QueryParameter parameter(Model model, String query, Symbol symbol) {
    Symbol typed = typed(model, symbol, new HashSet<>());
    Optional<Symbol.TypeFacts> facts = typed.typeFacts();
    String type = facts.flatMap(Symbol.TypeFacts::resolvedId).filter(id -> !id.isBlank())
        .or(() -> Optional.ofNullable(typed.metadata().get("type")))
        .orElse("");
    QueryParameter.Kind kind = QueryParameter.kindOf(facts.flatMap(Symbol.TypeFacts::primitive).orElse(""));
    boolean many = typed.multiplicity().map(bounds -> !"1".equals(bounds.upper().orElse("1"))).orElse(false);
    return new QueryParameter(query, symbol.name(), type, kind, many, hasValue(model, symbol.id()));
  }

  /** The symbol whose declaration types the parameter: itself, or the one it redefines or subsets. */
  private static Symbol typed(Model model, Symbol symbol, Set<String> visited) {
    boolean typedHere = symbol.typeFacts().flatMap(Symbol.TypeFacts::resolvedId).filter(id -> !id.isBlank()).isPresent()
        || symbol.metadata().containsKey("type");
    if (typedHere || !visited.add(symbol.id())) return symbol;
    for (Symbol.Specialization general : symbol.specializations()) {
      if (general.targetId().isEmpty() || !(general.kind().startsWith("redefin") || general.kind().startsWith("subset"))) continue;
      Optional<Symbol> target = model.findSymbol(general.targetId().get());
      if (target.isPresent()) return typed(model, target.get(), visited);
    }
    return symbol;
  }

  private static boolean hasValue(Model model, String parameter) {
    for (ElementProperties.Property property : model.elementProperties(parameter, "value").properties()) {
      if (property.name().equals("value") && !property.values().isEmpty()) return true;
    }
    return false;
  }
}
