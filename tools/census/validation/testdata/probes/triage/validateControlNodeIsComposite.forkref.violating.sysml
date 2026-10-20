// Census: validateControlNodeIsComposite
// Case: violating
// Ours: e[syntax] expected ';' or '{' after fork node
// Pilot: e: no viable alternative at input 'fork' | e: extraneous input '}' expecting EOF
action def A { fork ref f; }
