package B1 { part def X; attribute def X; }
package B0 { part def X; }
package B2 { alias X for B0::X; attribute def X; }
package B3 { part def <X> Y; attribute def X; }
package B4 { part def B { attribute def X; } part def C :> B { part def X; } }
package B5 { part def B1 { attribute def X; } part def B2 { part def X; } part def C :> B1, B2; }
package B6 { part def X { attribute def self; } part def Y { item def start; } action def Z { attribute def start; } }
package B7 { part def X { attribute self; } }
