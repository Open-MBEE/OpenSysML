// Census: validateRequirementConstraintMembershipIsComposite
// Case: violating
// Ours: e[syntax] expected ';' after 'require constraint'
// Pilot: e: no viable alternative at input 'require' | e: no viable alternative at input 'ref' | e: extraneous input '}' expecting EOF
requirement def R { require constraint ref c; }
