// Census: validateRequirementVerificationMembershipKind
// Case: conforming
// Ours: e[verification-owning-type] A requirement verification must be in the objective of a verification ca
// Pilot: e: no viable alternative at input 'verify' | e: extraneous input '}' expecting EOF
requirement def R; verification def V { verify requirement r : R; }
