import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
} from "@/components/ui/dialog";
import { expectNoClass } from "../helpers/dom";

// A dialog's surface, width, elevation and title tone are props
// (design-system §1.6), so a call site states what the dialog is and its
// className is left for layout.
type ContentProps = Pick<
  React.ComponentProps<typeof DialogContent>,
  "size" | "elevation"
>;

const renderDialog = (
  props: ContentProps = {},
  tone?: "default" | "destructive",
) => {
  render(
    <Dialog open onOpenChange={() => {}}>
      <DialogContent {...props}>
        <DialogTitle tone={tone}>Add User</DialogTitle>
        <DialogDescription>Create a new user account.</DialogDescription>
      </DialogContent>
    </Dialog>,
  );
  return screen.getByRole("dialog", { name: "Add User" });
};

const WIDTHS = ["sm:max-w-md", "sm:max-w-lg", "sm:max-w-xl", "sm:max-w-2xl"];

describe("DialogContent", () => {
  it("is a medium card with the raised shadow by default", () => {
    const dialog = renderDialog();

    expect(dialog).toHaveClass("bg-card", "border", "sm:max-w-md", "shadow-lg");
    expectNoClass(dialog, "bg-background", "shadow-floating");
  });

  it.each([
    ["md", "sm:max-w-md"],
    ["lg", "sm:max-w-lg"],
    ["xl", "sm:max-w-xl"],
    ["2xl", "sm:max-w-2xl"],
  ] as const)("is %s wide with %s", (size, width) => {
    const dialog = renderDialog({ size });

    expect(dialog).toHaveClass(width);
    for (const other of WIDTHS) {
      if (other !== width) expect(dialog).not.toHaveClass(other);
    }
  });

  it("takes the floating shadow instead of the raised one over media", () => {
    const dialog = renderDialog({ elevation: "floating" });

    expect(dialog).toHaveClass("shadow-floating");
    expect(dialog).not.toHaveClass("shadow-lg");
  });

  it("stays below the viewport width on small screens at every size", () => {
    expect(renderDialog({ size: "2xl" })).toHaveClass(
      "max-w-[calc(100%-2rem)]",
    );
  });

  it("describes itself with its description", () => {
    expect(renderDialog()).toHaveAccessibleDescription(
      "Create a new user account.",
    );
    expect(screen.getByText("Create a new user account.")).toHaveClass(
      "text-muted-foreground",
    );
  });
});

describe("DialogFooter", () => {
  // Call sites used to pass `gap-2 sm:gap-0`, which left the buttons
  // touching once they sat in a row.
  it("keeps a gap between its buttons, stacked or in a row", () => {
    render(
      <Dialog open onOpenChange={() => {}}>
        <DialogContent>
          <DialogTitle>Add User</DialogTitle>
          <DialogDescription>Create a new user account.</DialogDescription>
          <DialogFooter data-testid="footer">
            <button type="button">Cancel</button>
            <button type="button">Create User</button>
          </DialogFooter>
        </DialogContent>
      </Dialog>,
    );

    const footer = screen.getByTestId("footer");
    expect(footer).toHaveClass("gap-2", "flex-col-reverse", "sm:flex-row");
    expect(footer.className).not.toMatch(/(?:^|\s)(?:[a-z-]+:)*gap-0/);
  });
});

describe("DialogTitle tone", () => {
  it("takes the surrounding text colour by default", () => {
    renderDialog();

    const title = screen.getByRole("heading", { name: "Add User" });
    expect(title).not.toHaveClass("text-destructive");
  });

  it("is destructive when the action cannot be undone", () => {
    renderDialog({}, "destructive");

    expect(screen.getByRole("heading", { name: "Add User" })).toHaveClass(
      "text-destructive",
    );
  });
});

describe("AlertDialog", () => {
  const renderAlert = (tone?: "default" | "destructive") => {
    render(
      <AlertDialog open onOpenChange={() => {}}>
        <AlertDialogContent>
          <AlertDialogTitle tone={tone}>Delete playlist</AlertDialogTitle>
          <AlertDialogDescription>
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogContent>
      </AlertDialog>,
    );
    return screen.getByRole("alertdialog", { name: "Delete playlist" });
  };

  it("is a card at its one width", () => {
    const dialog = renderAlert();

    expect(dialog).toHaveClass("bg-card", "sm:max-w-lg", "shadow-lg");
    expect(dialog).not.toHaveClass("bg-background");
    expect(dialog).toHaveAccessibleDescription("This action cannot be undone.");
  });

  it("gives its title the destructive tone when asked", () => {
    renderAlert("destructive");

    expect(
      screen.getByRole("heading", { name: "Delete playlist" }),
    ).toHaveClass("text-destructive");
  });

  it("leaves the title plain otherwise", () => {
    renderAlert();

    expect(
      screen.getByRole("heading", { name: "Delete playlist" }),
    ).not.toHaveClass("text-destructive");
  });
});
