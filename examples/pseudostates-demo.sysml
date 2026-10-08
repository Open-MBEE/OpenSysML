package PseudostatesDemo {
    private import ScalarValues::*;
    private import StateMachines::*;
    
    // Pseudostates are written as the StateMachines library's metadata: an
    // ordinary state usage carrying an annotation, so the notation is standard
    // SysML v2. The deprecated keyword spellings (`choice evaluate;`,
    // `junction route;`) lower to the same graph.
    
    // Example 1: Choice (Dynamic Branching)
    // Guards evaluated at runtime when choice is entered
    state ChoiceExample {
        entry; then ready;
        state ready;
        state idle;
        #choice state evaluate;
        state pathA;
        state pathB;
        state pathC;
        
        succession first ready then idle;
        transition first idle then evaluate;
        transition first evaluate then pathA;
        transition first evaluate then pathB;
        transition first evaluate then pathC;
    }
    
    // Example 2: Junction (Static Branching)
    // Guards pre-evaluated before junction is entered
    state JunctionExample {
        entry; then ready;
        state ready;
        state receiving;
        #junction state route;
        state highPriority;
        state normalPriority;
        state lowPriority;
        
        succession first ready then receiving;
        transition first receiving then route;
        transition first route then highPriority;
        transition first route then normalPriority;
        transition first route then lowPriority;
    }
    
    // Key difference:
    // Choice: dynamic branching (guards evaluated when entered)
    // Junction: static branching (guards pre-evaluated)
}
