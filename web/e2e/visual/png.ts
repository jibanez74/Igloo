import { inflateSync } from "node:zlib";

// A PNG decoder for the screenshots Chromium writes: 8-bit RGB or RGBA, not
// interlaced. Just enough to compare two screenshots pixel by pixel without
// a tolerance; anything else is refused rather than guessed at.

const SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
const COLOR_TYPE_RGB = 2;
const COLOR_TYPE_RGBA = 6;

export type DecodedPng = {
  width: number;
  height: number;
  /** Bytes per pixel: 3 (RGB) or 4 (RGBA). */
  channels: number;
  /** `height` rows of `width * channels` bytes, unfiltered. */
  data: Buffer;
};

export type PixelDifference =
  | { kind: "same" }
  | { kind: "size"; expected: string; actual: string }
  | {
      kind: "pixels";
      count: number;
      /** The smallest box, in pixels, holding every difference. */
      box: { left: number; top: number; right: number; bottom: number };
    };

/** The Paeth predictor of the PNG specification's filter type 4. */
function paeth(left: number, up: number, upLeft: number) {
  const estimate = left + up - upLeft;
  const distanceLeft = Math.abs(estimate - left);
  const distanceUp = Math.abs(estimate - up);
  const distanceUpLeft = Math.abs(estimate - upLeft);

  if (distanceLeft <= distanceUp && distanceLeft <= distanceUpLeft) return left;
  return distanceUp <= distanceUpLeft ? up : upLeft;
}

export function decodePng(file: Buffer): DecodedPng {
  if (!file.subarray(0, SIGNATURE.length).equals(SIGNATURE)) {
    throw new Error("Not a PNG file");
  }

  let width = 0;
  let height = 0;
  let channels = 0;
  const compressed: Buffer[] = [];

  for (let offset = SIGNATURE.length; offset < file.length; ) {
    const length = file.readUInt32BE(offset);
    const type = file.toString("latin1", offset + 4, offset + 8);
    const body = file.subarray(offset + 8, offset + 8 + length);

    if (type === "IHDR") {
      width = body.readUInt32BE(0);
      height = body.readUInt32BE(4);
      const bitDepth = body[8];
      const colorType = body[9];
      const interlace = body[12];

      if (
        bitDepth !== 8 ||
        interlace !== 0 ||
        (colorType !== COLOR_TYPE_RGB && colorType !== COLOR_TYPE_RGBA)
      ) {
        throw new Error(
          `Unsupported PNG: bit depth ${bitDepth}, colour type ${colorType}, interlace ${interlace}`,
        );
      }
      channels = colorType === COLOR_TYPE_RGBA ? 4 : 3;
    } else if (type === "IDAT") {
      compressed.push(body);
    } else if (type === "IEND") {
      break;
    }

    // Length, type, body and CRC.
    offset += 12 + length;
  }

  if (channels === 0) throw new Error("PNG has no IHDR chunk");

  const stride = width * channels;
  const filtered = inflateSync(Buffer.concat(compressed));
  if (filtered.length !== height * (stride + 1)) {
    throw new Error("PNG image data has an unexpected length");
  }

  const data = Buffer.alloc(height * stride);
  for (let row = 0; row < height; row += 1) {
    const filter = filtered[row * (stride + 1)];
    const source = row * (stride + 1) + 1;
    const target = row * stride;

    for (let index = 0; index < stride; index += 1) {
      const left = index >= channels ? data[target + index - channels] : 0;
      const up = row > 0 ? data[target + index - stride] : 0;
      const upLeft =
        row > 0 && index >= channels
          ? data[target + index - stride - channels]
          : 0;
      const value = filtered[source + index];

      let predicted: number;
      switch (filter) {
        case 0:
          predicted = 0;
          break;
        case 1:
          predicted = left;
          break;
        case 2:
          predicted = up;
          break;
        case 3:
          predicted = (left + up) >> 1;
          break;
        case 4:
          predicted = paeth(left, up, upLeft);
          break;
        default:
          throw new Error(`Unknown PNG filter type ${filter}`);
      }

      data[target + index] = (value + predicted) & 0xff;
    }
  }

  return { width, height, channels, data };
}

/**
 * Compares two PNG files pixel by pixel, with no tolerance and no
 * anti-aliasing allowance. A missing alpha channel counts as opaque.
 */
export function comparePngs(expected: Buffer, actual: Buffer): PixelDifference {
  if (expected.equals(actual)) return { kind: "same" };

  const a = decodePng(expected);
  const b = decodePng(actual);
  if (a.width !== b.width || a.height !== b.height) {
    return {
      kind: "size",
      expected: `${a.width}x${a.height}`,
      actual: `${b.width}x${b.height}`,
    };
  }

  let count = 0;
  const box = { left: a.width, top: a.height, right: -1, bottom: -1 };

  for (let y = 0; y < a.height; y += 1) {
    for (let x = 0; x < a.width; x += 1) {
      const pixelA = (y * a.width + x) * a.channels;
      const pixelB = (y * b.width + x) * b.channels;
      const alphaA = a.channels === 4 ? a.data[pixelA + 3] : 255;
      const alphaB = b.channels === 4 ? b.data[pixelB + 3] : 255;

      if (
        a.data[pixelA] === b.data[pixelB] &&
        a.data[pixelA + 1] === b.data[pixelB + 1] &&
        a.data[pixelA + 2] === b.data[pixelB + 2] &&
        alphaA === alphaB
      ) {
        continue;
      }

      count += 1;
      box.left = Math.min(box.left, x);
      box.right = Math.max(box.right, x);
      box.top = Math.min(box.top, y);
      box.bottom = Math.max(box.bottom, y);
    }
  }

  return count === 0 ? { kind: "same" } : { kind: "pixels", count, box };
}
