export default function getImgSrc(path: string) {
  if (!path) {
    return "";
  }

  return path.startsWith("http")
    ? path
    : `${process.env.EXPO_PUBLIC_API_URL}${path}`;
}
