// Census: validateObjectiveMembershipIsComposite
// Case: violating
// Ours: e[reserved-keyword-name] "ref" is a reserved keyword, not a name the ID terminal admits; write 'r | e[syntax] expected '{' or ';' after declaration
// Pilot: e: no viable alternative at input 'objective' | e: extraneous input '}' expecting EOF
requirement def R; use case def U { objective ref o : R; }
