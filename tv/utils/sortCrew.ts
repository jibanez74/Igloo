import { jobRank, type Crew } from "../types/Crew";

export default function sortCrew(crew: Crew[], slice: boolean = true): Crew[] {
  const crewCopy = [...crew];

  crewCopy.sort((a, b) => {
    const rankA = jobRank[a.job] ?? Number.MAX_SAFE_INTEGER;
    const rankB = jobRank[b.job] ?? Number.MAX_SAFE_INTEGER;

    if (rankA === rankB) {
      return a.job.localeCompare(b.job);
    }

    return rankA - rankB;
  });

  return slice ? crewCopy.slice(0, 12) : crewCopy;
}
