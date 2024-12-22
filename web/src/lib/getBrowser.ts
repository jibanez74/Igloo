import Bowser from "bowser";
import type { Browser } from "../types/Platform";

export function getBrowser(): Browser {
  const browser = Bowser.getParser(window.navigator.userAgent);
  const browserInfo = browser.getBrowser();

  const result: Browser = {
    name: "unknown",
    version: "0",
    engine: "unknown",
  };

  switch (browserInfo.name?.toLowerCase()) {
    case "chrome": {
      result.name = "chrome";
      result.engine = "blink";
      break;
    }
    case "firefox": {
      result.name = "firefox";
      result.engine = "gecko";
      break;
    }
    case "safari": {
      result.name = "safari";
      result.engine = "webkit";
      break;
    }
    case "microsoft edge": {
      result.name = "edge";
      result.engine = "blink";
      break;
    }
  }

  if (browserInfo.version) {
    result.version = browserInfo.version;
  }

  return result;
}

export function compareVersions(version1: string, version2: string): number {
  // Parse versions into parts
  const v1Parts = version1.split(".").map(Number);
  const v2Parts = version2.split(".").map(Number);

  // Compare each part
  for (let i = 0; i < Math.max(v1Parts.length, v2Parts.length); i++) {
    const v1 = v1Parts[i] || 0;
    const v2 = v2Parts[i] || 0;
    if (v1 > v2) return 1;
    if (v1 < v2) return -1;
  }
  return 0;
}

// Example usage:
// const browser = getBrowser();
// console.log(`Browser: ${browser.name} ${browser.version}`);
// console.log(`Engine: ${browser.engine}`);

// Version comparison example:
// if (browser.name === 'chrome' && compareVersions(browser.version, '80.0') >= 0) {
//   console.log('Chrome version 80 or higher');
// }
