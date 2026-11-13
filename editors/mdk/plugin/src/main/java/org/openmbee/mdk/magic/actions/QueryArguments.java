package org.openmbee.mdk.magic.actions;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.openmbee.mdk.magic.engine.QueryParameter;
import org.openmbee.mdk.magic.engine.QueryPlan;
import org.openmbee.mdk.magic.identity.QualifiedNames;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Value;

/** Reads the bindings typed for "Run query": {@code name = value, value; …} or positional, each value by the parameter's type. */
public final class QueryArguments {
  private QueryArguments() {}

  /** Throws {@code IllegalArgumentException} when a name or position has no parameter, or a value does not read as its type. */
  public static Map<String, List<DocumentValue>> parse(String text, QueryPlan plan) {
    Map<String, List<DocumentValue>> bindings = new LinkedHashMap<>();
    List<String> assignments = CalcArguments.split(text, ';', "query arguments");
    for (int position = 0; position < assignments.size(); position++) {
      String assignment = assignments.get(position);
      if (assignment.isEmpty()) continue;
      int equals = assignmentSign(assignment);
      QueryParameter parameter;
      String value;
      if (equals > 0) {
        String name = assignment.substring(0, equals).trim();
        parameter = plan.parameter(name).orElseThrow(() -> new IllegalArgumentException(
            "no parameter named " + name + "; the query declares " + names(plan)));
        value = assignment.substring(equals + 1).trim();
      } else {
        if (position >= plan.parameters().size()) {
          throw new IllegalArgumentException(
              "argument " + (position + 1) + " has no parameter; the query declares " + names(plan));
        }
        parameter = plan.parameters().get(position);
        value = assignment;
      }
      bindings.put(plan.key(parameter), values(value, parameter));
    }
    return bindings;
  }

  /** The values of one parameter, separated by {@code ,}. */
  public static List<DocumentValue> values(String text, QueryParameter parameter) {
    List<DocumentValue> values = new ArrayList<>();
    for (String token : CalcArguments.split(text, ',', "query arguments")) values.add(value(token, parameter));
    if (values.size() > 1 && !parameter.many()) {
      throw new IllegalArgumentException("parameter " + parameter.name() + " takes one value, not " + values.size());
    }
    return values;
  }

  static DocumentValue value(String token, QueryParameter parameter) {
    Value scalar = CalcArguments.value(token);
    return switch (parameter.kind()) {
      case ELEMENT -> new DocumentValue.ElementRef(QualifiedNames.normalize(unquoted(scalar, token)), "");
      case STRING -> new DocumentValue.StringValue(unquoted(scalar, token));
      case INTEGER -> integer(token, parameter);
      case REAL -> {
        if (scalar instanceof Value.IntegerValue item) yield new DocumentValue.RealValue((double) item.value());
        if (scalar instanceof Value.RealValue item) yield new DocumentValue.RealValue(item.value());
        throw mismatch(token, parameter);
      }
      case BOOLEAN -> {
        if (scalar instanceof Value.BooleanValue item) yield new DocumentValue.BooleanValue(item.value());
        throw mismatch(token, parameter);
      }
    };
  }

  private static DocumentValue integer(String token, QueryParameter parameter) {
    try {
      return new DocumentValue.IntegerValue(Long.parseLong(token));
    } catch (NumberFormatException notLong) {
      try {
        return new DocumentValue.BigIntegerValue(new BigInteger(token));
      } catch (NumberFormatException notInteger) {
        throw mismatch(token, parameter);
      }
    }
  }

  private static String unquoted(Value scalar, String token) {
    return scalar instanceof Value.StringValue item ? item.value() : token;
  }

  private static IllegalArgumentException mismatch(String token, QueryParameter parameter) {
    return new IllegalArgumentException(
        "parameter " + parameter.name() + " takes " + parameter.type() + ", not " + token);
  }

  /** The {@code =} naming the parameter: before any quote, so a quoted value may hold one. */
  private static int assignmentSign(String assignment) {
    int equals = assignment.indexOf('=');
    int quote = assignment.indexOf('"');
    return quote >= 0 && quote < equals ? -1 : equals;
  }

  private static String names(QueryPlan plan) {
    if (plan.parameters().isEmpty()) return "no parameters";
    return String.join(", ", plan.parameters().stream().map(QueryParameter::name).toList());
  }
}
