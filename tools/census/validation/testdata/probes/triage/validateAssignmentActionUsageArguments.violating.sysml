// Census: validateAssignmentActionUsageArguments
// Case: violating
// Ours: e[syntax] expected ':=' after assignment target | e[syntax] expected a body member
// Pilot: e: no viable alternative at input 'assign' | e: extraneous input '}' expecting EOF
action def A { attribute x; assign x; }
