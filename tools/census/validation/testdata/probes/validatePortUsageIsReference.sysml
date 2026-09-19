// Census: validatePortUsageIsReference
// Expect: error: A port usage must be referential.
package P { port def PD; port def Q { variation port vp : PD { variant port a : PD; } } }
