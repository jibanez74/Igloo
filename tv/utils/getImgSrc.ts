export default function getImgSrc(path: string) {
  if (!path) {
    return "";
  }

  return path.startsWith("http") ? path : `/api/v1${path}`;
}
