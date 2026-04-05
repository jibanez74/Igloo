type WebkitDocument = Document & {
  webkitFullscreenElement?: Element | null;
  webkitExitFullscreen?: () => void;
  webkitFullscreenEnabled?: boolean;
};

type WebkitHTMLElement = HTMLElement & {
  webkitRequestFullscreen?: () => void;
};

type WebkitHTMLVideoElement = HTMLVideoElement & {
  webkitEnterFullscreen?: () => void;
  webkitExitFullscreen?: () => void;
};

export function getFullscreenElement(): Element | null {
  const doc = document as WebkitDocument;
  return document.fullscreenElement ?? doc.webkitFullscreenElement ?? null;
}

export function exitDocumentFullscreen(): Promise<void> {
  const doc = document as WebkitDocument;
  const exit =
    document.exitFullscreen?.bind(document) ??
    doc.webkitExitFullscreen?.bind(document);
  if (!exit) {
    return Promise.resolve();
  }
  const result = exit();
  if (
    result !== undefined &&
    typeof (result as Promise<void>).then === "function"
  ) {
    return result as Promise<void>;
  }
  return Promise.resolve();
}

export function requestElementFullscreen(el: HTMLElement): Promise<void> {
  const node = el as WebkitHTMLElement;
  const req =
    el.requestFullscreen?.bind(el) ?? node.webkitRequestFullscreen?.bind(el);
  if (!req) {
    return Promise.reject(new Error("Fullscreen API not available"));
  }
  const result = req();
  if (
    result !== undefined &&
    typeof (result as Promise<void>).then === "function"
  ) {
    return result as Promise<void>;
  }
  return Promise.resolve();
}

export function canRequestElementFullscreen(el: HTMLElement): boolean {
  const node = el as WebkitHTMLElement;
  return (
    typeof el.requestFullscreen === "function" ||
    typeof node.webkitRequestFullscreen === "function"
  );
}

// Permissive heuristic: accept either the standard or WebKit fullscreen flag.
export function isDocumentFullscreenEntryLikely(): boolean {
  const doc = document as WebkitDocument;
  if (document.fullscreenEnabled || doc.webkitFullscreenEnabled) {
    return true;
  }
  return document.fullscreenEnabled !== false && doc.webkitFullscreenEnabled !== false;
}

export function tryWebKitVideoEnterFullscreen(video: HTMLVideoElement): boolean {
  const v = video as WebkitHTMLVideoElement;
  if (typeof v.webkitEnterFullscreen !== "function") {
    return false;
  }
  v.webkitEnterFullscreen();
  return true;
}

export function tryWebKitVideoExitFullscreen(video: HTMLVideoElement): boolean {
  const v = video as WebkitHTMLVideoElement;
  if (typeof v.webkitExitFullscreen !== "function") {
    return false;
  }
  v.webkitExitFullscreen();
  return true;
}
