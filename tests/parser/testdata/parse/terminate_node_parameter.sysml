package Terminates {
    action def ProcessWorkflow;
    action def Workflow {
        ref action process : ProcessWorkflow;
        first start;
        then action terminate1 terminate process;
        then action terminate2 terminate process { doc /* ends the process */ }
        then action terminate3 terminate;
        then done;
    }
}
