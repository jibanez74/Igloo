import API_URL from "@/constants/Backend";

export async function fetchGet(url: string) {
  if (!url) {
    throw new Error("no url provided to fetch call");
  }

  return fetch(API_URL + url);
}
