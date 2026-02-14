const months = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

// takes in a date string and returns a formatted date string
// format is month day, year
export function formatDate(date: string) {
  const d = new Date(date);

  return `${months[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
}

// takes in a duration in milliseconds and returns a formatted duration string
// the format is hours:minutes:seconds
export function formatDuration(ms: number) {
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }

  return `${minutes}m ${seconds}s`;
}

// takes in a duration in milliseconds and returns a formatted duration string
export function formatTrackDuration(ms: number) {
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function formatBitRate(bitRate: number) {
  return `${Math.round(bitRate / 1000)} kbps`;
}

// Format seconds into mm:ss format (for audio player progress)
// Handles edge cases like NaN and Infinity
export function formatTimeSeconds(seconds: number) {
  if (!isFinite(seconds) || isNaN(seconds)) return "0:00";

  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);

  return `${mins}:${secs.toString().padStart(2, "0")}`;
}
