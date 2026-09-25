// Census: validateAnalysisCaseUsageType
// Expect: error: An analysis case must be typed by one analysis case definition.
package P { analysis def A1; analysis def A2; analysis a : A1, A2; }
