export const passwordCases = [
  { name: "72 ASCII bytes", password: "a".repeat(72) },
  { name: "72 multibyte bytes", password: "é".repeat(36) },
  { name: "72 emoji bytes", password: "🔒".repeat(18) },
  { name: "nine characters", password: "abcdefghi" },
  { name: "nine emoji", password: "🔒".repeat(9) },
  { name: "literal whitespace and combining characters", password: " ée\u0301🔒abcd " },
  { name: "73 ASCII bytes", password: "a".repeat(73), error: "must be at most 72 UTF-8 bytes." },
  { name: "73 multibyte bytes", password: "é".repeat(36) + "a", error: "must be at most 72 UTF-8 bytes." },
  { name: "74 multibyte bytes", password: "é".repeat(37), error: "must be at most 72 UTF-8 bytes." },
  { name: "76 emoji bytes", password: "🔒".repeat(19), error: "must be at most 72 UTF-8 bytes." },
  { name: "eight characters", password: "abcdefgh", error: "must be at least 9 characters." },
  { name: "eight emoji", password: "🔒".repeat(8), error: "must be at least 9 characters." },
];
