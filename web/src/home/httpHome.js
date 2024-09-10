export async function getLatestMovies() {
  try {
    const response = await fetch("/api/v1/movie/latest", {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();

    return data.Movies;
  } catch (error) {
    console.error("Error fetching latest movies:", error);
    throw error;
  }
}
