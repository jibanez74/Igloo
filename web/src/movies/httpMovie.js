export async function getMovie(id) {
  try {
    const response = await fetch(`/api/v1/movie/${id}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();

    return data.Movie;
  } catch (error) {
    console.error("Error fetching latest movies:", error);
    throw error;
  }
}

export async function cancelTranscodeProcess(pid) {
  await fetch(`/api/v1/transcode/cancel/${pid}`);
}
