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

  it("keeps input foregrounds legible on the dark glass field", () => {
    // In dark mode ui/input.tsx's `dark:bg-input/30` overrides the light
    // background, so every foreground color needs a dark variant.
    expect(lightInputClassName).toContain(
      lightInputColorClass("dark:text", "50"),
    );
    expect(lightInputClassName).toContain(
      lightInputColorClass("dark:placeholder:text", "400"),
    );
    expect(lightInputActionClassName).toContain(
      lightInputColorClass("dark:text", "400"),
    );
    expect(lightInputActionClassName).toContain(
      lightInputColorClass("dark:hover:text", "100"),
    );
    expect(lightInputPeerHoverClassName).toContain(
      lightInputColorClass("dark:peer-hover:text", "50"),
    );
    expect(lightInputPeerHoverClassName).not.toContain(
      lightInputColorClass("dark:peer-hover:bg", "100/95"),
    );
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

    expect(source).toContain("focus-visible:ring-ring");
    expect(source).not.toContain("ring-primary-foreground");
  });
});
