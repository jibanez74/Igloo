import API_URL from "../constants/Backend"

export default function getImgSrc(path: string) {
  if (!path) {
    return "";
  }

  return path.startsWith("http") ? path : API_URL + path;
}
