/** Actions bound to the window-wide keyboard shortcuts. */
export interface Shortcuts {
  save?(): void; // Ctrl/Cmd+S
  next?(): void; // F7
  prev?(): void; // Shift+F7
  applyAll?(): void; // Ctrl/Cmd+Shift+A
  cancel?(): void; // Esc
  refresh?(): void; // F5, Ctrl/Cmd+Enter
}

/**
 * Installs the shortcuts on the window, in the capture phase so they work
 * whichever pane has focus and win over editor and webview defaults (e.g.
 * F7 caret browsing). While a dialog is open its own keys apply instead.
 */
export function installShortcuts(s: Shortcuts): void {
  window.addEventListener(
    "keydown",
    (e) => {
      if (document.querySelector("dialog[open]")) return;
      const mod = e.ctrlKey || e.metaKey;
      const key = e.key.length === 1 ? e.key.toLowerCase() : e.key;
      let action: (() => void) | undefined;
      if (mod && !e.shiftKey && !e.altKey && key === "s") action = s.save;
      else if (mod && e.shiftKey && !e.altKey && key === "a") action = s.applyAll;
      else if (!mod && !e.altKey && key === "F7") action = e.shiftKey ? s.prev : s.next;
      else if (!mod && !e.shiftKey && !e.altKey && key === "Escape") action = s.cancel;
      else if (!e.shiftKey && !e.altKey && ((!mod && key === "F5") || (mod && key === "Enter"))) action = s.refresh;
      if (!action) return;
      e.preventDefault();
      e.stopPropagation();
      action();
    },
    true,
  );
}
