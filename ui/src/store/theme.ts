import { logError } from "@/lib/log";
import { signal } from "@preact/signals-core";
import { getSettingsPreferences, patchSettingsPreferences } from "@/api";

export type Theme = "light" | "dark";
export type ColorTheme = "default" | "zinc" | "rose" | "green" | "violet";
export type Palette =
  | "breeze"
  | "github-dark"
  | "solarized"
  | "dracula"
  | "nord"
  | "monokai"
  | "catppuccin"
  | "tokyo-night"
  | "one-dark"
  | "gruvbox"
  | "rose-pine";

export interface ThemePreset {
  id: string;
  label: string;
  palette: Palette;
  mode: Theme;
  color: ColorTheme;
  colorHex: string;
  description: string;
}

function p(
  id: string,
  label: string,
  palette: Palette,
  mode: Theme,
  color: ColorTheme,
  colorHex: string,
  description: string,
): ThemePreset {
  return { id, label, palette, mode, color, colorHex, description };
}

export const THEME_PRESETS: ThemePreset[] = [
  // Breeze palette: core presets
  p(
    "light",
    "Light Blue",
    "breeze",
    "light",
    "default",
    "#3b82f6",
    "Clean & classic",
  ),
  p(
    "paper",
    "Warm Light",
    "breeze",
    "light",
    "zinc",
    "#c4a882",
    "Warm & focused",
  ),
  p(
    "dark",
    "Dark Blue",
    "breeze",
    "dark",
    "default",
    "#1d4ed8",
    "Classic dark",
  ),
  p(
    "noir",
    "Dark Neutral",
    "breeze",
    "dark",
    "zinc",
    "#6b7280",
    "Monochrome & sleek",
  ),

  // External palettes
  p(
    "github-dark",
    "GitHub Dark",
    "github-dark",
    "dark",
    "default",
    "#2ea043",
    "Dark with green accent",
  ),
  p(
    "solarized-light",
    "Solarized Light",
    "solarized",
    "light",
    "default",
    "#b58900",
    "Warm light palette",
  ),
  p(
    "solarized-dark",
    "Solarized Dark",
    "solarized",
    "dark",
    "default",
    "#b58900",
    "Warm dark palette",
  ),
  p(
    "dracula",
    "Dracula",
    "dracula",
    "dark",
    "default",
    "#bd93f9",
    "Dracula theme",
  ),
  p(
    "nord",
    "Nord",
    "nord",
    "dark",
    "default",
    "#88c0d0",
    "Arctic, bluish dark",
  ),
  p(
    "monokai",
    "Monokai",
    "monokai",
    "dark",
    "default",
    "#a9dc76",
    "Monokai Pro dark",
  ),
  p(
    "catppuccin-latte",
    "Catppuccin Latte",
    "catppuccin",
    "light",
    "default",
    "#ea76cb",
    "Catppuccin light",
  ),
  p(
    "catppuccin-mocha",
    "Catppuccin Mocha",
    "catppuccin",
    "dark",
    "default",
    "#cba6f7",
    "Catppuccin dark",
  ),
  p(
    "tokyo-night",
    "Tokyo Night",
    "tokyo-night",
    "dark",
    "default",
    "#7aa2f7",
    "Tokyo Night",
  ),
  p(
    "one-dark",
    "One Dark",
    "one-dark",
    "dark",
    "default",
    "#61afef",
    "Atom's iconic dark",
  ),
  p(
    "gruvbox",
    "Gruvbox Dark",
    "gruvbox",
    "dark",
    "default",
    "#fabd2f",
    "Retro groove",
  ),
  p(
    "gruvbox-light",
    "Gruvbox Light",
    "gruvbox",
    "light",
    "default",
    "#d79921",
    "Retro light",
  ),
  p(
    "rose-pine",
    "Rosé Pine",
    "rose-pine",
    "dark",
    "default",
    "#c4a7e7",
    "Rosé Pine",
  ),
  p(
    "rose-pine-dawn",
    "Rosé Pine Dawn",
    "rose-pine",
    "light",
    "default",
    "#907aa9",
    "Rosé Pine dawn",
  ),
];

export const theme = signal<Theme>("dark");
export const colorTheme = signal<ColorTheme>("default");
export const palette = signal<Palette>("breeze");
export const currentPreset = signal<string>("dark");

function applyAttr(name: string, value: string): void {
  document.documentElement.dataset[name] = value;
}

export function initTheme(serverPreset?: string): void {
  applyAttr("palette", "breeze");

  // Priority: server > localStorage > legacy localStorage > defaults
  if (serverPreset) {
    const match = THEME_PRESETS.find((p) => p.id === serverPreset);
    if (match) {
      applyPresetInternal(match);
      return;
    }
  }

  const storedPreset = localStorage.getItem("theme-preset");
  if (storedPreset) {
    const match = THEME_PRESETS.find((p) => p.id === storedPreset);
    if (match) {
      applyPresetInternal(match);
      return;
    }
  }

  const stored = localStorage.getItem("theme") as Theme | null;
  const t = stored ?? "dark";
  const storedColor = localStorage.getItem("color-theme") as ColorTheme | null;
  const c = storedColor ?? "default";

  theme.value = t;
  colorTheme.value = c;
  palette.value = "breeze";

  applyAttr("palette", "breeze");
  applyAttr("theme", t);
  applyAttr("color", c);

  const fallback = THEME_PRESETS.find((p) => p.mode === t && p.color === c);
  currentPreset.value = fallback?.id ?? (t === "dark" ? "dark" : "light");
  localStorage.setItem("theme-preset", currentPreset.value);
}

/** Load theme from server preferences. Call after auth is confirmed. */
export async function loadThemeFromPreferences(): Promise<void> {
  try {
    const { data } = await getSettingsPreferences({ throwOnError: true });
    const prefs = data;
    if (!prefs?.theme) return;
    if (prefs.theme === currentPreset.value) return;
    const match = THEME_PRESETS.find((p) => p.id === prefs.theme);
    if (match) applyPresetInternal(match);
  } catch {
    // ignore
  }
}

function applyPresetInternal(preset: ThemePreset): void {
  currentPreset.value = preset.id;
  palette.value = preset.palette;
  theme.value = preset.mode;
  colorTheme.value = preset.color;

  applyAttr("palette", preset.palette);
  applyAttr("theme", preset.mode);
  applyAttr("color", preset.color);

  localStorage.setItem("theme-preset", preset.id);
  localStorage.setItem("theme", preset.mode);
  localStorage.setItem("color-theme", preset.color);
}

export function applyPreset(id: string): void {
  const preset = THEME_PRESETS.find((p) => p.id === id);
  if (!preset) return;
  applyPresetInternal(preset);
  saveThemeToServer(id);
}

export function toggleTheme(): void {
  if (palette.value !== "breeze") return;

  const next: Theme = theme.value === "light" ? "dark" : "light";
  const match = THEME_PRESETS.find((p) =>
    p.palette === "breeze" && p.mode === next && p.color === colorTheme.value
  );
  if (!match) return;

  applyPresetInternal(match);
  saveThemeToServer(match.id);
}

function saveThemeToServer(presetId: string): void {
  patchSettingsPreferences({
    body: { theme: presetId },
    throwOnError: true,
  }).catch((err) => logError("Failed to save theme:", err));
}

export function setColorTheme(c: ColorTheme): void {
  colorTheme.value = c;
  applyAttr("color", c);
  localStorage.setItem("color-theme", c);

  const match = THEME_PRESETS.find((p) =>
    p.palette === "breeze" && p.mode === theme.value && p.color === c
  );
  if (match) {
    currentPreset.value = match.id;
    localStorage.setItem("theme-preset", match.id);
  }
}
