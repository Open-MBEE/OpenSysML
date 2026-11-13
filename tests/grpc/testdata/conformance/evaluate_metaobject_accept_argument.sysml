package Accepts {
  private import ScalarValues::*;
  port def P;
  part def Machine {
    port p : P;
    state sm { entry; then idle; state idle; transition t first idle accept sig : Integer via p then idle; }
  }
  attribute receiverArgument [*] = (Machine::sm::t meta SysML::TransitionUsage).triggerAction.receiverArgument;
}
