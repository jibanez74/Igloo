import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MoviePlaybackStatusScreen from "@/components/movies/MoviePlaybackStatusScreen";

describe("MoviePlaybackStatusScreen", () => {
  it("announces error variants with role=alert", () => {
    render(
      <MoviePlaybackStatusScreen
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
      <MoviePlaybackStatusScreen variant="loading" message="Preparing playback" />,
    );

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText("Preparing playback")).toBeInTheDocument();
  });
});
