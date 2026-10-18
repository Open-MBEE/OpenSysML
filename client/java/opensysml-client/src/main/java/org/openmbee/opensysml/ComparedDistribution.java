package org.openmbee.opensysml;

/**
 * The statistics of one observable's values, from a tool's results or from runs.
 *
 * @param count how many values the distribution holds
 * @param min the smallest value
 * @param mean the arithmetic mean
 * @param p50 the median
 * @param p90 the 90th percentile
 * @param max the largest value
 * @param deviation the sample standard deviation
 */
public record ComparedDistribution(
    long count, double min, double mean, double p50, double p90, double max, double deviation) {}
