import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import PlaybackStatusScreen from "@/components/playback/PlaybackStatusScreen";

describe("PlaybackStatusScreen", () => {
  it("announces error variants with role=alert", () => {
    render(
      <PlaybackStatusScreen
        title="Playback failed"
        message="The stream could not be played."
      />,
    );

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Playback failed");
    expect(alert).toHaveTextContent("The stream could not be played.");
  });

  it("keeps the loading variant out of the alert channel", () => {
    render(
      <PlaybackStatusScreen variant="loading" message="Preparing playback" />,
    );

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText("Preparing playback")).toBeInTheDocument();
  });

  // The pair every full-screen status shares: the call to action in the
  // primary fill with the glow, the quiet second action in outline, both the
  // 44 px xl pill.
  it("draws its actions as the xl pill pair", () => {
    render(
      <PlaybackStatusScreen
        message="The stream could not be played."
        actions={[
          { id: "retry", label: "Try again", ariaLabel: "Try again", onClick: () => {}, icon: "retry" },
          { id: "back", label: "Go back", ariaLabel: "Go back", onClick: () => {}, icon: "back", variant: "secondary" },
        ]}
      />,
    );

    const retry = screen.getByRole("button", { name: "Try again" });
    expect(retry).toHaveAttribute("data-variant", "default");
    expect(retry).toHaveAttribute("data-size", "xl");
    expect(retry).toHaveAttribute("data-shape", "pill");
    expect(retry).toHaveClass("shadow-glow");

    const back = screen.getByRole("button", { name: "Go back" });
    expect(back).toHaveAttribute("data-variant", "outline");
    expect(back).toHaveAttribute("data-size", "xl");
    expect(back).not.toHaveClass("shadow-glow");
  });
});
