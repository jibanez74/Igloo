export default function getImgSrc(path: string) {
  if (!path) {
    return "";
  }

  return path.startsWith("http")
    ? path
    : `https://swifty.hare-crocodile.ts.net${path}`;
}
