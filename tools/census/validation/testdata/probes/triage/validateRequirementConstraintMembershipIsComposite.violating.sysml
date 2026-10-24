// Census: validateRequirementConstraintMembershipIsComposite
// Case: violating
// Ours: e[syntax] expected ';' after require expression
// Pilot: e: no viable alternative at input 'require' | e: extraneous input '}' expecting EOF
requirement def R { require ref constraint c; }
