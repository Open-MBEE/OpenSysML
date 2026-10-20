// Census: validateSendActionParameters
// Case: violating
// Ours: e[send-payload-missing] a send action written as a state subaction or a transition effect must h
// Pilot: e: no viable alternative at input 'state' | e: no viable alternative at input 'entry' | e: missing '}' at 'd'
package P { item def D; state def S { state a { entry send to d; } item d : D; } }
