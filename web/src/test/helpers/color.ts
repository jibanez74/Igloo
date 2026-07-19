// Dependency-free color utilities for the contrast test.
// Converts OKLCH (the form used by our CSS tokens) to linear sRGB and computes
// WCAG 2.x relative-luminance contrast ratios.

export interface Oklch {
  L: number;
  C: number;
  H: number;
}

/** OKLCH -> linear sRGB (Björn Ottosson's matrices). Values may fall outside [0,1]. */
function oklchToLinearRgb({ L, C, H }: Oklch): [number, number, number] {
  const a = C * Math.cos((H * Math.PI) / 180);
  const b = C * Math.sin((H * Math.PI) / 180);

  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;

  const l = l_ ** 3;
  const m = m_ ** 3;
  const s = s_ ** 3;

  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ];
}

const clamp01 = (c: number) => Math.min(1, Math.max(0, c));

/** WCAG relative luminance. In-gamut linear values are the WCAG-linearized channels. */
export function relativeLuminance(color: Oklch): number {
  const [r, g, b] = oklchToLinearRgb(color).map(clamp01);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG contrast ratio between two OKLCH colors (>= 1). */
export function contrastRatio(a: Oklch, b: Oklch): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const [hi, lo] = la > lb ? [la, lb] : [lb, la];
  return (hi + 0.05) / (lo + 0.05);
}

/** OKLCH -> uppercase #RRGGBB (sRGB gamma-encoded, clamped to gamut). */
export function oklchToHex(color: Oklch): string {
  const channels = oklchToLinearRgb(color).map((v) => {
    const c = clamp01(v);
    const encoded = c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055;
    return Math.round(clamp01(encoded) * 255);
  });
  return `#${channels
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("")}`.toUpperCase();
}

/**
 * Parse the `--token: oklch(L C H[ / a]);` declarations inside a CSS block.
 * Tokens whose value carries an alpha channel are skipped (not opaque text pairs).
 */
export function parseOklchTokens(cssBlock: string): Record<string, Oklch> {
  const out: Record<string, Oklch> = {};
  const re = /(--[\w-]+):\s*oklch\(([^)]+)\)/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(cssBlock)) !== null) {
    const [name, raw] = [match[1], match[2]];
    if (raw.includes("/")) continue; // has alpha — skip for contrast pairs
    const parts = raw.trim().split(/\s+/).map(Number);
    if (parts.length < 3 || parts.some((n) => Number.isNaN(n))) continue;
    out[name] = { L: parts[0], C: parts[1], H: parts[2] };
  }
  return out;
}

/** Extract the body of a top-level CSS rule by selector (e.g. ":root", ".dark"). */
export function extractRuleBody(css: string, selector: string): string {
  const start = css.indexOf(selector + " {");
  if (start === -1) throw new Error(`selector ${selector} not found`);
  const open = css.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < css.length; i++) {
    if (css[i] === "{") depth++;
    else if (css[i] === "}") {
      depth--;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  throw new Error(`unterminated rule for ${selector}`);
}
