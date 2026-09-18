// Census: validateSendActionUsagePayloadArgument
// Expect: error: a send action written as a state subaction or a transition effect must have a payload
package P { item def D; state def S { state a { entry send to d; } item d : D; } }
