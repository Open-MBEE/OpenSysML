// SysML v2 State Machine Execution Demo
// Demonstrates event-driven state machine execution with TimeEvent, ChangeEvent, guards, and hierarchical states

package StateMachineDemo {
    
    // Example 1: Simple Traffic Light (TimeEvent transitions)
    state TrafficLightSimple {
        entry start red;
        
        state red {
            entry do { x = 0; }  // Initialize counter
        }
        
        state green {
            entry do { x = x + 1; }
        }
        
        state yellow {
            entry do { x = x + 1; }
        }
        
        done end off;
        
        // TimeEvent transitions (after duration in seconds)
        transition red to green after 30.0;
        transition green to yellow after 25.0;
        transition yellow to red after 5.0;
        
        // Exit to final state after one cycle
        transition red to off when x > 2;
    }
    
    // Example 2: State Machine with Guards (conditional transitions)
    state ProcessingSystem {
        entry start idle;
        
        state idle {
            entry do { counter = 0; }
        }
        
        state processing {
            entry do { counter = counter + 1; }
        }
        
        state error;
        
        done end complete;
        
        // Guarded TimeEvent transitions
        transition idle to processing after 1.0 if counter < 5;
        transition idle to complete after 1.0 if counter >= 5;
        
        transition processing to idle after 2.0 if counter < 3;
        transition processing to error after 2.0 if counter >= 3;
        
        transition error to idle after 1.0;
    }
    
    // Example 3: Hierarchical States (composite states with nested substates)
    state WorkflowSystem {
        entry start workflow;
        
        // Composite state with substates
        state workflow {
            entry start ready;
            
            state ready {
                entry do { taskCount = 0; }
            }
            
            // Nested composite state
            state processing {
                entry start validate;
                
                state validate {
                    entry do { taskCount = taskCount + 1; }
                }
                
                state execute {
                    entry do { taskCount = taskCount + 1; }
                }
                
                transition validate to execute after 2.0;
            }
            
            transition ready to processing after 1.0;
        }
        
        done end done;
        
        // Exit from nested state to final
        transition workflow to done when taskCount >= 2;
    }
    
    // Example 4: ChangeEvent (condition-based transitions)
    state MonitoringSystem {
        entry start monitoring;
        
        state monitoring {
            entry do { temperature = 20.0; }
        }
        
        state alarm {
            entry do { alarmCount = alarmCount + 1; }
        }
        
        state shutdown;
        
        done end stopped;
        
        // ChangeEvent transition (fires when condition becomes true)
        transition monitoring to alarm when temperature > 100.0;
        transition alarm to monitoring after 5.0 if alarmCount < 3;
        transition alarm to shutdown after 5.0 if alarmCount >= 3;
        
        transition shutdown to stopped after 1.0;
    }
    
    // Example 5: Transition with Effect Actions
    state CounterSystem {
        entry start counting;
        
        state counting {
            entry do { n = 0; }
        }
        
        state doubled;
        
        done end finished;
        
        // Transition with effect action (executed during transition)
        transition counting to doubled after 1.0 do { result = n * 2; };
        transition doubled to finished after 1.0;
    }
}

// REPL Usage Examples:
//
// 1. Load this file:
//    %load examples/state-machine-demo.sysml
//
// 2. Execute simple traffic light:
//    %state TrafficLightSimple
//    %advance    # red -> green
//    %advance    # green -> yellow
//    %advance    # yellow -> red
//    %current    # show current state, time, data
//
// 3. Debug processing system:
//    %state ProcessingSystem
//    %events     # show event queue
//    %advance    # step through transitions
//    %current    # inspect state variables
//
// 4. Explore hierarchical states:
//    %state WorkflowSystem
//    %advance    # workflow.ready -> workflow.processing.validate
//    %current    # see stateStack showing active configuration
//    %advance    # workflow.processing.validate -> workflow.processing.execute
//    %advance    # exit to done (when taskCount >= 2)
//
// 5. Monitor with ChangeEvent:
//    %state MonitoringSystem
//    %current    # temperature = 20.0
//    # (In real system, temperature would change via external input)
//    # ChangeEvent would fire when temperature > 100.0
//
// 6. Stop debugging:
//    %stop
