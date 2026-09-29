package org.openmbee.opensysml;

import java.util.Objects;

/**
 * The question a verification asks: an evaluation of the declared or held values (the default), or
 * a {@code holds} or {@code satisfiable} question the service's solvers answer — a question other
 * than {@link #QUESTION_EVALUATE} needs {@code verification_questions}.
 *
 * @param question the question, as the service spells it
 */
public record VerifyOptions(String question) {

  /** The evaluation a verification asks unless a question names another. */
  public static final String QUESTION_EVALUATE = "evaluate";

  /** The question whether the claim holds for every assignment the free features can take. */
  public static final String QUESTION_HOLDS = "holds";

  /** The question whether any free assignment satisfies the claim. */
  public static final String QUESTION_SATISFIABLE = "satisfiable";

  /**
   * Creates options asking a question.
   *
   * @param question the question, never {@code null}
   */
  public VerifyOptions {
    Objects.requireNonNull(question, "question");
  }

  /**
   * The default options: an evaluation.
   *
   * @return options asking {@link #QUESTION_EVALUATE}
   */
  public static VerifyOptions defaults() {
    return new VerifyOptions(QUESTION_EVALUATE);
  }

  /**
   * Options asking one question.
   *
   * @param question the question: one of {@link #QUESTION_EVALUATE}, {@link #QUESTION_HOLDS} or
   *     {@link #QUESTION_SATISFIABLE}
   * @return options asking it
   */
  public static VerifyOptions asking(String question) {
    return new VerifyOptions(question);
  }
}
