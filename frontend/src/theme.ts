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
 * Applies the system theme now (before the settings have loaded) and returns
 * a function that sets the "theme" setting. The system preference is
 * followed while the setting is "system".
 */
export function installTheme(root: ThemeRoot, dark: DarkQuery): (setting: string | undefined) => void {
  let current: string | undefined = "system";
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
