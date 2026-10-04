package A1 { part def X; item def X; }
package A2 { part def X; part def X; }
package A3 { part x; part x; }
package A4 { part def <X> Y; part def X; }
package A5a { part def X; }
package A5 { alias X for A5a::X; part def X; }
package A6 { part def B { attribute def X; } part def C :> B { attribute def X; } }
package A7 { part def B1 { attribute def X; } part def B2 { attribute def X; } part def C :> B1, B2; }
