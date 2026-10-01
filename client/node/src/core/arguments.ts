// Guards the public entry points reach before any call: a non-string argument
// is refused as a TypeError, and source text a lone UTF-16 surrogate makes
// unsendable is refused as a RangeError, as Python's client refuses them.

// One half of a surrogate pair without the other (a high surrogate followed by
// anything but a low surrogate, or a low surrogate not led by a high one).
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/;

/** Refuses `value` that is not a string, naming the parameter it was passed as. */
export function requireString(param: string, value: unknown): asserts value is string {
  if (typeof value !== "string") {
    throw new TypeError(`${param} must be a string, got ${value === null ? "null" : typeof value}`);
  }
}

/** Refuses `value` that is not a string, or one whose lone surrogates cannot be sent. */
export function requireSourceText(param: string, value: unknown): asserts value is string {
  requireString(param, value);
  if (LONE_SURROGATE.test(value)) {
    throw new RangeError(`${param} contains a lone UTF-16 surrogate, which cannot be sent to the service`);
  }
}
