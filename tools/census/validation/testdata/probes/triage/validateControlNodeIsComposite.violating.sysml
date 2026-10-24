// Census: validateControlNodeIsComposite
// Case: violating
// Ours: e[syntax] expected a body member
// Pilot: e: no viable alternative at input 'ref' | e: extraneous input '}' expecting EOF
action def A { ref fork f; }
