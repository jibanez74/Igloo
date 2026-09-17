const BOOTSTRAP_TITLE_ATTR = "data-igloo-bootstrap-title";
const BOOTSTRAP_DESCRIPTION_ATTR = "data-igloo-bootstrap-description";

type BootstrapHeadState = {
  titleText: string | null;
  descriptionContent: string | null;
  titleNode: HTMLTitleElement | null;
  descriptionNode: HTMLMetaElement | null;
};

const bootstrapHeadState: BootstrapHeadState = {
  titleText: null,
  descriptionContent: null,
  titleNode: null,
  descriptionNode: null,
};

export function syncBootstrapHeadMetadata() {
  if (typeof document === "undefined") {
    return;
  }

  const bootstrapTitle = document.head.querySelector<HTMLTitleElement>(
    `title[${BOOTSTRAP_TITLE_ATTR}]`,
  );
  const bootstrapDescription = document.head.querySelector<HTMLMetaElement>(
    `meta[name="description"][${BOOTSTRAP_DESCRIPTION_ATTR}]`,
  );

  if (bootstrapTitle) {
    bootstrapHeadState.titleNode = bootstrapTitle;
    if (bootstrapHeadState.titleText === null) {
      bootstrapHeadState.titleText = bootstrapTitle.textContent ?? "";
    }
  }

  if (bootstrapDescription) {
    bootstrapHeadState.descriptionNode = bootstrapDescription;
    if (bootstrapHeadState.descriptionContent === null) {
      bootstrapHeadState.descriptionContent =
        bootstrapDescription.getAttribute("content") ?? "";
    }
  }

  const nonBootstrapTitles = document.head.querySelectorAll(
    `title:not([${BOOTSTRAP_TITLE_ATTR}])`,
  );
  const nonBootstrapDescriptions = document.head.querySelectorAll(
    `meta[name="description"]:not([${BOOTSTRAP_DESCRIPTION_ATTR}])`,
  );

  if (nonBootstrapTitles.length > 0) {
    bootstrapTitle?.remove();
  } else if (bootstrapHeadState.titleText !== null) {
    const titleNode =
      bootstrapTitle ??
      bootstrapHeadState.titleNode ??
      document.createElement("title");

    titleNode.setAttribute(BOOTSTRAP_TITLE_ATTR, "");
    titleNode.textContent = bootstrapHeadState.titleText;

    if (titleNode.parentNode !== document.head) {
      document.head.append(titleNode);
    }

    bootstrapHeadState.titleNode = titleNode;
  }

  if (nonBootstrapDescriptions.length > 0) {
    bootstrapDescription?.remove();
  } else if (bootstrapHeadState.descriptionContent !== null) {
    const descriptionNode =
      bootstrapDescription ??
      bootstrapHeadState.descriptionNode ??
      document.createElement("meta");

    descriptionNode.setAttribute("name", "description");
    descriptionNode.setAttribute(
      BOOTSTRAP_DESCRIPTION_ATTR,
      "",
    );
    descriptionNode.setAttribute(
      "content",
      bootstrapHeadState.descriptionContent,
    );

    if (descriptionNode.parentNode !== document.head) {
      document.head.append(descriptionNode);
    }

    bootstrapHeadState.descriptionNode = descriptionNode;
  }
}
