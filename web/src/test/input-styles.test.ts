import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import {
  lightInputActionClassName,
  lightInputClassName,
  lightInputPeerHoverClassName,
} from "@/lib/input-styles";

const lightInputColorClass = (prefix: string, shade: string) =>
  `${prefix}-slate-${shade}`;

describe("light input styles", () => {
  it("keeps light-input actions dark on light surfaces", () => {
    expect(lightInputClassName).toContain(lightInputColorClass("bg", "50/92"));
    expect(lightInputClassName).toContain(lightInputColorClass("text", "950"));
    expect(lightInputActionClassName).toContain(
      lightInputColorClass("text", "500"),
    );
    expect(lightInputActionClassName).toContain(
      lightInputColorClass("hover:text", "800"),
    );
    expect(lightInputActionClassName).not.toContain("hover:text-foreground");
  });

  it("keeps peer-hover states in the light input palette", () => {
    expect(lightInputPeerHoverClassName).toContain(
      lightInputColorClass("peer-hover:bg", "100/95"),
    );
    expect(lightInputPeerHoverClassName).toContain(
      lightInputColorClass("peer-hover:text", "950"),
    );
    expect(lightInputPeerHoverClassName).not.toContain("peer-hover:bg-foreground");
  });

  it("uses the theme focus ring for album play overlays", () => {
    const source = readFileSync(
      resolve(process.cwd(), "src/components/AlbumCard.tsx"),
      "utf8",
    );

    expect(source).toContain("focus:ring-ring");
    expect(source).not.toContain("focus:ring-primary-foreground");
  });
});
