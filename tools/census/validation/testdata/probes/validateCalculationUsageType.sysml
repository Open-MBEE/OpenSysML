// Census: validateCalculationUsageType
// Expect: error: A calculation must be typed by one calculation definition.
package P { calc def C1 { return : ScalarValues::Integer; 1 } calc def C2 { return : ScalarValues::Integer; 2 } calc c : C1, C2; }
