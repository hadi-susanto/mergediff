// Light/dark theme: the "theme" setting (system | light | dark) decides the
// data-theme attribute on <html>, which styles.css keys its colours on.
// DOM-free apart from the small interfaces below, so it is unit-tested.

export type Theme = "light" | "dark";

/** The theme to show for a setting value and the system preference. */
export function resolveTheme(setting: string | undefined, systemDark: boolean): Theme {
  if (setting === "light" || setting === "dark") return setting;
  return systemDark ? "dark" : "light";
}

/** The parts of <html> the theme touches. */
export interface ThemeRoot {
  dataset: { theme?: string };
  style: { colorScheme: string };
}

/** The parts of a MediaQueryList for "(prefers-color-scheme: dark)". */
export interface DarkQuery {
  matches: boolean;
  addEventListener(type: "change", listener: () => void): void;
}

/**
 * The "theme" setting from the page URL. The backend opens the window at
 * /?theme=<setting>, so the right theme is known before the first paint
 * (index.html applies it inline; this keeps theme.ts in agreement).
 */
export function themeFromURL(search: string): string | undefined {
  return new URLSearchParams(search).get("theme") ?? undefined;
}

/**
 * Applies the initial setting now (from the URL, or the system theme) and
 * returns a function that changes it. The system preference is followed
 * while the setting is "system".
 */
export function installTheme(
  root: ThemeRoot,
  dark: DarkQuery,
  initial: string | undefined = "system",
): (setting: string | undefined) => void {
  let current: string | undefined = initial;
  const apply = () => {
    const theme = resolveTheme(current, dark.matches);
    root.dataset.theme = theme;
    root.style.colorScheme = theme;
  };
  dark.addEventListener("change", apply);
  apply();
  return (setting) => {
    current = setting;
    apply();
  };
}
