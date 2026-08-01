// Action Executor Demo - Showcasing Token Flow Execution
// This file demonstrates the action executor capabilities implemented in Phase 2

package ActionExecutorDemo {
    
    // Demo 1: Sequential execution (Initial → Action → Final)
    action def SequentialAction {
        action start : InitialNode;
        action compute : ActionExecutionNode {
            in x : Integer;
            return x * 2;
        }
        action end : FinalNode;
        
        succession start then compute then end;
    }
    
    // Demo 2: Fork/Join (Parallel execution)
    action def ParallelAction {
        action start : InitialNode;
        action fork : ForkNode;
        action task1 : ActionExecutionNode {
            return 10;
        }
        action task2 : ActionExecutionNode {
            return 20;
        }
        action task3 : ActionExecutionNode {
            return 30;
        }
        action join : JoinNode;
        action end : FinalNode;
        
        succession start then fork;
        succession fork then task1;
        succession fork then task2;
        succession fork then task3;
        succession task1 then join;
        succession task2 then join;
        succession task3 then join;
        succession join then end;
    }
    
    // Demo 3: Decision/Merge (Conditional branching)
    action def ConditionalAction {
        action start : InitialNode;
        action decision : DecisionNode;
        action pathA : ActionExecutionNode {
            return "took path A";
        }
        action pathB : ActionExecutionNode {
            return "took path B";
        }
        action merge : MergeNode;
        action end : FinalNode;
        
        succession start then decision;
        flow decision to pathA if (x > 10);
        flow decision to pathB if (x <= 10);
        succession pathA then merge;
        succession pathB then merge;
        succession merge then end;
    }
    
    // Demo 4: Complex workflow (Fork → Decision → Merge → Join)
    action def ComplexWorkflow {
        action start : InitialNode;
        action fork : ForkNode;
        
        // Branch 1: Decision
        action decision1 : DecisionNode;
        action branch1A : ActionExecutionNode {
            return "branch1A";
        }
        action branch1B : ActionExecutionNode {
            return "branch1B";
        }
        action merge1 : MergeNode;
        
        // Branch 2: Decision
        action decision2 : DecisionNode;
        action branch2A : ActionExecutionNode {
            return "branch2A";
        }
        action branch2B : ActionExecutionNode {
            return "branch2B";
        }
        action merge2 : MergeNode;
        
        action join : JoinNode;
        action final : ActionExecutionNode {
            return "workflow complete";
        }
        action end : FinalNode;
        
        succession start then fork;
        
        // Branch 1
        succession fork then decision1;
        flow decision1 to branch1A if (x > 50);
        flow decision1 to branch1B; // else
        succession branch1A then merge1;
        succession branch1B then merge1;
        succession merge1 then join;
        
        // Branch 2
        succession fork then decision2;
        flow decision2 to branch2A if (x > 25);
        flow decision2 to branch2B; // else
        succession branch2A then merge2;
        succession branch2B then merge2;
        succession merge2 then join;
        
        succession join then final;
        succession final then end;
    }
}
