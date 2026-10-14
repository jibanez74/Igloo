import { fireEvent, screen, waitFor, within, render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Route } from "@/routes/_auth/people/$id";
import type { PersonData } from "@/types/people";
import { requestURL, jsonResponse } from "../helpers/api";
import { authUserData } from "../helpers/fixtures";
import { personProfile, personTitle } from "../helpers/people";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

function mockPerson(profile: PersonData | (() => PersonData), status = 200) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);
    if (url === "/api/auth/user") return jsonResponse({ error: false, data: { user: authUserData() } });
    if (url === "/api/people/9001") return jsonResponse(status === 200 ? { error: false, data: typeof profile === "function" ? profile() : profile } : { error: true, message: "Unavailable" }, status);
    return jsonResponse({ error: false, data: { unread_count: 0 } });
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("person route", () => {
  it("renders biography paragraphs, facts, grouped roles and external destinations", async () => {
    mockPerson(personProfile());
    await renderRoute("/people/9001");
    expect(await screen.findByRole("heading", { name: "Alex Vega", level: 1 })).toBeVisible();
    expect(readDocumentHead().title).toBe("Alex Vega - Igloo");
    expect(screen.getByText("First paragraph.")).toBeVisible();
    expect(screen.getByText("Second paragraph.")).toBeVisible();
    expect(screen.getByText("Non-binary")).toBeVisible();
    expect(screen.getByText("Alexandra Vega")).toBeVisible();
    expect(screen.queryByText("Death date")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to Home" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Alex Vega on IMDb (opens in a new tab)" })).toHaveAttribute("href", "https://www.imdb.com/name/nm9001");
    const filmography = screen.getByRole("region", { name: "Filmography" });
    expect(within(filmography).getByRole("link", { name: /Remote Movie on TMDB/ })).toHaveAttribute("target", "_blank");
    expect(within(filmography).getAllByText("Acting: Hero")).toHaveLength(3);
    expect(within(filmography).getByText("Director")).toBeVisible();
    expect(within(filmography).getByRole("link", { name: /Library Show/ })).toHaveAttribute("href", "/tv-shows/401");
    expect(screen.getByRole("link", { name: /View full-size photo/ })).toHaveAttribute("href", "/api/tmdb/images/original/extra.jpg");
    const portrait = screen.getAllByRole("img", { name: "Photo of Alex Vega" })[0];
    fireEvent.error(portrait);
    expect(screen.getByRole("img", { name: "No photo available for Alex Vega" })).toBeVisible();
  });
  it("discloses a long biography using the keyboard", async () => {
    const data = personProfile(); data.person.biography = `${"Long biography sentence. ".repeat(40)}\n\nFinal paragraph.`;
    mockPerson(data); await renderRoute("/people/9001");
    const toggle = await screen.findByRole("button", { name: "Read more" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Final paragraph.")).not.toBeInTheDocument();
    toggle.focus(); await userEvent.keyboard("{Enter}");
    expect(screen.getByRole("button", { name: "Show less" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Final paragraph.")).toBeVisible();
    await userEvent.keyboard(" ");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });
  it("omits missing facts/photos and labels an empty biography and portrait", async () => {
    const data = personProfile({ filmography: [], photos: [] });
    data.person = { ...data.person, biography: "", birthday: "", deathday: "", place_of_birth: "", gender: 0, also_known_as: [], profile_path: "" };
    mockPerson(data); await renderRoute("/people/9001?returnTo=https%3A%2F%2Fevil.test");
    expect(await screen.findByText("No biography available.")).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Personal information" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Photos" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Known for" })).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: "No photo available for Alex Vega" })).toBeVisible();
    expect(screen.getByText("No credits match these filters.")).toBeVisible();
  });
  it("keeps URL filters, paginates at 24, and resets the page when filters change", async () => {
    const data = personProfile({ filmography: Array.from({ length: 50 }, (_, i) => personTitle(i + 1, { title: `Career ${String(i).padStart(2, "0")}`, media_type: i < 25 ? "movie" : "tv" })) });
    mockPerson(data); const { router } = await renderRoute("/people/9001?page=2&returnTo=%2Ftv-shows%2F401%3Fseason%3D2");
    const section = await screen.findByRole("region", { name: "Filmography" });
    expect(within(section).getAllByRole("link")).toHaveLength(24);
    expect(screen.getByRole("link", { name: "Back" })).toHaveAttribute("href", "/tv-shows/401?season=2");
    fireEvent.change(screen.getByLabelText("Media type"), { target: { value: "tv" } });
    await waitFor(() => expect(router.state.location.search).toMatchObject({ media: "tv" }));
    expect(router.state.location.search.page).toBeUndefined();
    expect(within(section).getAllByRole("link")).toHaveLength(24);
    fireEvent.change(screen.getByLabelText("Work"), { target: { value: "crew" } });
    expect(await screen.findByText("No credits match these filters.")).toBeVisible();
  });
  it("retries partial profiles without hiding local data", async () => {
    let available = false;
    mockPerson(() => personProfile({ enrichment_status: available ? "available" : "unavailable" }));
    await renderRoute("/people/9001");
    expect(await screen.findByText(/TMDB is temporarily unavailable/)).toBeVisible();
    expect(screen.getByRole("heading", { name: "Alex Vega", level: 1 })).toBeVisible();
    available = true;
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(screen.queryByText(/TMDB is temporarily unavailable/)).not.toBeInTheDocument());
  });
  it("shows not-found, recoverable errors and a matching skeleton", async () => {
    const Pending = Route.options.pendingComponent!;
    const view = render(<Pending />);
    expect(screen.getByRole("status", { name: "Loading person profile" })).toBeVisible();
    view.unmount();
    mockPerson(personProfile(), 404); await renderRoute("/people/9001");
    expect(await screen.findByRole("heading", { name: "Person not found" })).toBeVisible();
  });
});
