package LoopTests {
    private import ScalarValues::*;
    action def WhileLoop {
        in calc whileTest { true }
    }
    action def CountedLoop :> WhileLoop {
        attribute count : Integer = 0;
        in calc redefines whileTest { count < 3 }
    }
    part p {
        attribute a : Integer = 3;
        in calc scaled { a * 5 }
    }
}
