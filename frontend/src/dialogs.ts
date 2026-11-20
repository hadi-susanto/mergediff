// Modal dialogs built on <dialog>, so they look the same on every webview.

function makeDialog(message: string): HTMLDialogElement {
  const dialog = document.createElement("dialog");
  dialog.className = "md-dialog";
  const text = document.createElement("p");
  text.textContent = message;
  dialog.appendChild(text);
  return dialog;
}

function makeButton(label: string, cls: string, onClick: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.className = `btn ${cls}`;
  b.textContent = label;
  b.addEventListener("click", onClick);
  return b;
}

/** Shows an error message; onClose runs after it is dismissed. */
export function showError(message: string, onClose: () => void = () => {}): void {
  const dialog = makeDialog(message);
  const row = document.createElement("div");
  row.className = "md-dialog-buttons";
  row.appendChild(makeButton("Close", "btn-primary", () => dialog.close()));
  dialog.appendChild(row);
  dialog.addEventListener("close", () => {
    dialog.remove();
    onClose();
  });
  document.body.appendChild(dialog);
  dialog.showModal();
}

/**
 * Asks a yes/no question. Resolves true for the confirm button, false for
 * the cancel button or Escape.
 */
export function confirmDialog(message: string, confirmLabel: string, cancelLabel: string): Promise<boolean> {
  return new Promise((resolve) => {
    const dialog = makeDialog(message);
    let result = false;
    const row = document.createElement("div");
    row.className = "md-dialog-buttons";
    row.append(
      makeButton(cancelLabel, "", () => dialog.close()),
      makeButton(confirmLabel, "btn-primary", () => {
        result = true;
        dialog.close();
      }),
    );
    dialog.appendChild(row);
    dialog.addEventListener("close", () => {
      dialog.remove();
      resolve(result);
    });
    document.body.appendChild(dialog);
    dialog.showModal();
  });
}
