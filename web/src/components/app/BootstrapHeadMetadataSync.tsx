import { useLayoutEffect } from "react";
import { useLocation } from "@tanstack/react-router";
import { syncBootstrapHeadMetadata } from "@/lib/bootstrap-head-metadata";

export default function BootstrapHeadMetadataSync() {
  const href = useLocation({
    select: location => location.href,
  });

  useLayoutEffect(() => {
    syncBootstrapHeadMetadata();
  }, [href]);

  useLayoutEffect(() => {
    const observer = new MutationObserver(records => {
      const shouldSync = records.some(record =>
        [...record.addedNodes, ...record.removedNodes].some(node => {
          if (node instanceof HTMLTitleElement) {
            return true;
          }

          return (
            node instanceof HTMLMetaElement &&
            node.getAttribute("name") === "description"
          );
        }),
      );

      if (shouldSync) {
        syncBootstrapHeadMetadata();
      }
    });

    observer.observe(document.head, {
      childList: true,
    });

    return () => {
      observer.disconnect();
    };
  }, []);

  return null;
}
