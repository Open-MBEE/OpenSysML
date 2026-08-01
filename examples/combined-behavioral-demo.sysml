// SysML v2 Combined Action & State Machine Demo
// Demonstrates coordination between actions and state machines

package CombinedBehavioralDemo {
    
    // Example 1: State Machine with Action Entry Behaviors
    // State entry actions execute inline expressions
    state DataProcessor {
        entry start init;
        
        state init {
            // Entry action: initialize data
            entry do { 
                input = 100;
                processed = 0;
            }
        }
        
        state process {
            // Entry action: compute during state entry
            entry do {
                processed = input * 2 + 10;
            }
        }
        
        state validate {
            // Entry action: check result
            entry do {
                valid = processed > 200;
            }
        }
        
        done end complete;
        
        transition init to process after 1.0;
        transition process to validate after 1.0;
        transition validate to complete after 1.0;
    }
    
    // Example 2: Action with Complex Control Flow
    // Actions support fork/join, decision, merge
    action ParallelProcessing {
        first start initial;
        
        // Fork: create parallel execution paths
        fork fork1;
        
        action processA {
            in input;
            return input + 10;
        }
        
        action processB {
            in input;
            return input + 20;
        }
        
        action processC {
            in input;
            return input + 30;
        }
        
        // Join: synchronize parallel paths
        join join1;
        
        action aggregate {
            in processA;
            in processB;
            in processC;
            return processA + processB + processC;
        }
        
        done end final;
        
        // Control flow
        then initial fork1;
        then fork1 processA;
        then fork1 processB;
        then fork1 processC;
        then processA join1;
        then processB join1;
        then processC join1;
        then join1 aggregate;
        then aggregate final;
        
        // Object flow: connect pins
        flow initial.input to processA.input;
        flow initial.input to processB.input;
        flow initial.input to processC.input;
        flow processA.processA to aggregate.processA;
        flow processB.processB to aggregate.processB;
        flow processC.processC to aggregate.processC;
    }
    
    // Example 3: Action with Decision/Merge
    // Conditional branching based on guards
    action ConditionalWorkflow {
        first start initial;
        
        action computeValue {
            in x;
            return x * 2;
        }
        
        decision decision1;
        
        action pathA {
            return 100;
        }
        
        action pathB {
            return 200;
        }
        
        merge merge1;
        
        action finalize {
            in result;
            return result + 1;
        }
        
        done end final;
        
        // Control flow
        then initial computeValue;
        then computeValue decision1;
        then decision1 pathA if computeValue > 10;  // Guard
        then decision1 pathB if computeValue <= 10; // Guard
        then pathA merge1;
        then pathB merge1;
        then merge1 finalize;
        then finalize final;
    }
    
    // Example 4: Hierarchical State Machine with Multiple Regions
    // (Note: Orthogonal regions are future work, but hierarchy works)
    state MultilevelWorkflow {
        entry start system;
        
        // Top-level composite state
        state system {
            entry start subsystemA;
            
            // Nested composite state
            state subsystemA {
                entry start taskA1;
                
                state taskA1 {
                    entry do { counter = 0; }
                }
                
                state taskA2 {
                    entry do { counter = counter + 1; }
                }
                
                transition taskA1 to taskA2 after 2.0;
            }
            
            state subsystemB {
                entry start taskB1;
                
                state taskB1 {
                    entry do { counter = counter + 10; }
                }
            }
            
            // Exit from subsystemA to subsystemB
            transition subsystemA to subsystemB after 5.0;
        }
        
        done end shutdown;
        
        // Exit from system to final
        transition system to shutdown when counter > 10;
    }
    
    // Example 5: Real-World Workflow
    // Order processing system with states and actions
    state OrderProcessing {
        entry start received;
        
        state received {
            entry do {
                orderId = 1001;
                status = "received";
            }
        }
        
        state validating {
            entry do {
                // Inline validation logic
                valid = orderId > 1000;
                status = "validating";
            }
        }
        
        state processing {
            entry do {
                processed = valid;
                status = "processing";
            }
        }
        
        state shipped {
            entry do {
                status = "shipped";
            }
        }
        
        state cancelled;
        
        done end completed;
        
        // Happy path
        transition received to validating after 1.0;
        transition validating to processing after 2.0 if valid;
        transition processing to shipped after 3.0;
        transition shipped to completed after 1.0;
        
        // Error path
        transition validating to cancelled after 2.0 if !valid;
        transition cancelled to completed after 1.0;
    }
}

// REPL Usage Examples:
//
// 1. State machine with entry actions:
//    %state DataProcessor
//    %advance          # init (input=100, processed=0)
//    %current          # show state data
//    %advance          # process (processed=210)
//    %advance          # validate (valid=true)
//    %advance          # complete
//
// 2. Parallel action workflow:
//    %action ParallelProcessing
//    %step             # initial token
//    %step             # fork creates 3 tokens
//    %tokens           # show parallel tokens at processA, processB, processC
//    %continue         # run to completion
//    # Result: (110 + 120 + 130) = 360
//
// 3. Conditional action workflow:
//    %action ConditionalWorkflow
//    %step             # initial
//    %step             # computeValue (result = x*2)
//    %step             # decision evaluates guards
//    %tokens           # token at pathA or pathB depending on guard
//    %continue         # merge and finalize
//
// 4. Hierarchical state debugging:
//    %state MultilevelWorkflow
//    %current          # stateStack: [system, subsystemA, taskA1]
//    %advance          # subsystemA.taskA1 -> subsystemA.taskA2
//    %current          # stateStack shows active configuration
//    %advance          # exit subsystemA, enter subsystemB
//    %current          # stateStack: [system, subsystemB, taskB1]
//
// 5. Order processing workflow:
//    %state OrderProcessing
//    %advance          # received -> validating
//    %current          # orderId=1001, valid=true
//    %advance          # validating -> processing (guard passes)
//    %advance          # processing -> shipped
//    %advance          # shipped -> completed
//
// 6. Compare action vs state execution:
//    # Actions: token-flow (fork/join parallelism, immediate execution)
//    # States: event-driven (time-based scheduling, persistent state)
//    %action ParallelProcessing    # Petri-net semantics
//    %state OrderProcessing        # State machine semantics
