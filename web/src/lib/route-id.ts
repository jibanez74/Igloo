// Route id params arrive as strings. `parseInt` stops at the first character it
// cannot read, so "12abc" and "12/../x" both yield 12 and a malformed URL would
// load a real record. Require the whole parameter to be a positive integer and
// return null otherwise, so each route falls through to its not-found branch.
export function parseRouteId(value: string): number | null {
  if (!/^[1-9]\d*$/.test(value)) return null;

  const id = Number(value);

  return Number.isSafeInteger(id) ? id : null;
}
