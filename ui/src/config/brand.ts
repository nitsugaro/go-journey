import { palettes, paletteLabels } from './palettes';

export { createPalette, defaultCustomPaletteSeed, paletteLabels, palettes } from './palettes';
export type { BrandColorName, BuiltInPaletteName, ThemePalette, ThemePaletteSeed } from './palettes';

export const brand = {
  appName: 'Go Journey Studio',
  tagline: 'Configure journeys, scripts and runtime caches',
  defaultPalette: 'default',
  palettes,
  paletteLabels,
} as const;
