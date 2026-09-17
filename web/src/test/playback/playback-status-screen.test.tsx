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
});
