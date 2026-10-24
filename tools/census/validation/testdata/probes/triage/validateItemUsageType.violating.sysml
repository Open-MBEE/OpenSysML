// Census: validateItemUsageType
// Case: violating
// Ours: w[name-conflict] Duplicate of inherited member name 'self' from DataValue, Item | e[usage-typing] An occurrence, item or part must be typed by occurrence definitions.
// Pilot: e: An occurrence, item or part must be typed by occurrence definitions.
attribute def AD; part def P { item i : AD; }
