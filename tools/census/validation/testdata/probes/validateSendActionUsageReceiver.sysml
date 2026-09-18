// Census: validateSendActionUsageReceiver
// Expect: warning: sending to the port p should use 'via' rather than 'to'
package P { item def Sig; port def Pt; part def Q { port p : Pt; action a { send new Sig() to p; } } }
