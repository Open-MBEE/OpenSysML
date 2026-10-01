package org.openmbee.opensysml;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashSet;
import java.util.List;

/** Ranks candidate names by Ratcliff/Obershelp similarity, as Python's difflib does. */
final class NearNames {

  private static final double CUTOFF = 0.6;

  private NearNames() {}

  static List<String> closest(String word, List<String> candidates, int limit) {
    record Scored(double score, String name) {}
    List<Scored> scored = new ArrayList<>();
    for (String candidate : new LinkedHashSet<>(candidates)) {
      double score = ratio(candidate, word);
      if (score >= CUTOFF) {
        scored.add(new Scored(score, candidate));
      }
    }
    scored.sort(
        Comparator.comparingDouble(Scored::score)
            .thenComparing(Scored::name)
            .reversed());
    return scored.stream().limit(limit).map(Scored::name).toList();
  }

  static double ratio(String a, String b) {
    int total = a.length() + b.length();
    return total == 0 ? 1.0 : 2.0 * matches(a, 0, a.length(), b, 0, b.length()) / total;
  }

  private static int matches(String a, int aLo, int aHi, String b, int bLo, int bHi) {
    int bestI = aLo;
    int bestJ = bLo;
    int bestSize = 0;
    int[] previous = new int[bHi - bLo + 1];
    for (int i = aLo; i < aHi; i++) {
      int[] current = new int[bHi - bLo + 1];
      for (int j = bLo; j < bHi; j++) {
        if (a.charAt(i) == b.charAt(j)) {
          int size = previous[j - bLo] + 1;
          current[j - bLo + 1] = size;
          if (size > bestSize) {
            bestSize = size;
            bestI = i - size + 1;
            bestJ = j - size + 1;
          }
        }
      }
      previous = current;
    }
    if (bestSize == 0) {
      return 0;
    }
    return bestSize
        + matches(a, aLo, bestI, b, bLo, bestJ)
        + matches(a, bestI + bestSize, aHi, b, bestJ + bestSize, bHi);
  }
}
