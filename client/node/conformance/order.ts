/**
 * Orders strings by code unit, as the Go and Java harnesses do; locale-aware
 * collation would report a different sequence for the same scenario.
 */
export function byCodeUnit(a: string, b: string): number {
  if (a < b) {
    return -1;
  }
  return a > b ? 1 : 0;
}
