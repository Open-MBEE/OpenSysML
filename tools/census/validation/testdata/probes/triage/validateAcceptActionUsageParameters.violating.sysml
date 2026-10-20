// Census: validateAcceptActionUsageParameters
// Case: violating
// Ours: e[syntax] expected the payload of the accept: a type (`accept Warning`), a named p
// Pilot: e: no viable alternative at input 'action' | e: no viable alternative at input 'accept'
action def A { action a { accept; } }
