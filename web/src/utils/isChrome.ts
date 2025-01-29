export default function isChrome(): boolean {
  const userAgent = navigator.userAgent.toLowerCase();
  const isChrome =
    /chrome/.test(userAgent) && !/edg|edge|opr|opera/.test(userAgent);

  const isGoogleVendor =
    navigator.vendor?.toLowerCase().includes("google") ?? false;

  return isChrome && isGoogleVendor;
}
