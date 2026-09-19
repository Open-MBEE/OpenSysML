// Census: validateTransitionUsageTriggerActions
// Expect: error: A transition with an accepter must have a state as its source.
package P {
  item def A;
  state def S {
    entry action init;
    transition init accept A then s1;
    state s1;
  }
}
