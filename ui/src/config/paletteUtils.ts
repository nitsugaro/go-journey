export type ThemePalette = {
  bg: string;
  surface: string;
  surfaceOverlay: string;
  surfaceTranslucent: string;
  surfaceMutedTransparent: string;
  surfaceMuted: string;
  surfaceSubtle: string;
  surfaceSoft: string;
  border: string;
  borderSubtle: string;
  borderSoft: string;
  ink: string;
  muted: string;
  mutedSoft: string;
  mutedFaint: string;
  disabled: string;
  blue: string;
  blueSoft: string;
  blueSubtle: string;
  blueBorder: string;
  blueSelection: string;
  green: string;
  greenSoft: string;
  greenBorder: string;
  red: string;
  redSoft: string;
  redBorder: string;
  warning: string;
  warningSoft: string;
  warningBorder: string;
  overlay: string;
  white: string;
  codeKeyword: string;
  codeAtom: string;
  codeNumber: string;
  codeString: string;
  codeVariable: string;
  codeProperty: string;
  codeOperator: string;
  [key: string]: string;
};

export type ThemePaletteSeed = {
  bg: string;
  surface: string;
  ink: string;
  accent: string;
  success: string;
  danger: string;
  warning?: string;
};

export const defaultCustomPaletteSeed: ThemePaletteSeed = {
  bg: '#f6f8fb',
  surface: '#ffffff',
  ink: '#101828',
  accent: '#2557a7',
  success: '#16805d',
  danger: '#c03744',
};

export function createPalette(seed: ThemePaletteSeed): ThemePalette {
  const warning = seed.warning ?? '#d97706';

  return {
    bg: seed.bg,
    surface: seed.surface,
    surfaceOverlay: alpha(seed.surface, 0.97),
    surfaceTranslucent: alpha(mix(seed.surface, seed.bg, 0.28), 0.9),
    surfaceMutedTransparent: alpha(mix(seed.accent, seed.surface, 0.11), 0.88),
    surfaceMuted: mix(seed.accent, seed.surface, 0.12),
    surfaceSubtle: mix(seed.bg, seed.surface, 0.42),
    surfaceSoft: mix(seed.ink, seed.surface, 0.06),
    border: mix(seed.accent, seed.surface, 0.28),
    borderSubtle: mix(seed.ink, seed.surface, 0.16),
    borderSoft: mix(seed.accent, seed.surface, 0.14),
    ink: seed.ink,
    muted: mix(seed.ink, seed.surface, 0.58),
    mutedSoft: mix(seed.ink, seed.surface, 0.42),
    mutedFaint: mix(seed.ink, seed.surface, 0.28),
    disabled: mix(seed.ink, seed.surface, 0.24),
    blue: seed.accent,
    blueSoft: mix(seed.accent, seed.surface, 0.18),
    blueSubtle: mix(seed.accent, seed.surface, 0.08),
    blueBorder: mix(seed.accent, seed.surface, 0.38),
    blueSelection: alpha(mix(seed.accent, seed.surface, 0.34), 0.75),
    green: seed.success,
    greenSoft: mix(seed.success, seed.surface, 0.16),
    greenBorder: mix(seed.success, seed.surface, 0.36),
    red: seed.danger,
    redSoft: mix(seed.danger, seed.surface, 0.16),
    redBorder: mix(seed.danger, seed.surface, 0.36),
    warning,
    warningSoft: mix(warning, seed.surface, 0.16),
    warningBorder: mix(warning, seed.surface, 0.36),
    overlay: alpha(seed.ink, 0.56),
    white: '#ffffff',
    codeKeyword: seed.accent,
    codeAtom: seed.danger,
    codeNumber: mix(seed.accent, seed.danger, 0.48),
    codeString: seed.success,
    codeVariable: mix(seed.ink, seed.surface, 0.62),
    codeProperty: mix(seed.accent, seed.success, 0.46),
    codeOperator: seed.danger,
    node1Stroke: seed.accent,
    node1Soft: mix(seed.accent, seed.surface, 0.18),
    node1Text: seed.accent,
    node2Stroke: mix(seed.accent, '#0891b2', 0.58),
    node2Soft: mix(mix(seed.accent, '#0891b2', 0.58), seed.surface, 0.17),
    node2Text: mix(seed.accent, '#0891b2', 0.68),
    node3Stroke: '#0891b2',
    node3Soft: mix('#0891b2', seed.surface, 0.16),
    node3Text: '#0e7490',
    node4Stroke: seed.success,
    node4Soft: mix(seed.success, seed.surface, 0.16),
    node4Text: seed.success,
    node5Stroke: warning,
    node5Soft: mix(warning, seed.surface, 0.16),
    node5Text: warning,
    node6Stroke: mix(warning, seed.danger, 0.58),
    node6Soft: mix(mix(warning, seed.danger, 0.58), seed.surface, 0.16),
    node6Text: mix(warning, seed.danger, 0.65),
    node7Stroke: '#7c3aed',
    node7Soft: mix('#7c3aed', seed.surface, 0.15),
    node7Text: '#6d28d9',
    node8Stroke: '#4f46e5',
    node8Soft: mix('#4f46e5', seed.surface, 0.15),
    node8Text: '#4338ca',
    node9Stroke: mix(seed.accent, '#7c3aed', 0.45),
    node9Soft: mix(mix(seed.accent, '#7c3aed', 0.45), seed.surface, 0.15),
    node9Text: mix(seed.accent, '#7c3aed', 0.55),
    node10Stroke: mix(seed.ink, seed.surface, 0.52),
    node10Soft: mix(seed.ink, seed.surface, 0.08),
    node10Text: mix(seed.ink, seed.surface, 0.6),
    node11Stroke: '#0ea5e9',
    node11Soft: mix('#0ea5e9', seed.surface, 0.15),
    node11Text: '#0284c7',
    node12Stroke: '#f59e0b',
    node12Soft: mix('#f59e0b', seed.surface, 0.16),
    node12Text: '#b45309',
  };
}

export type BrandColorName = keyof ThemePalette;

function mix(hexA: string, hexB: string, weight: number) {
  const a = hexToRgb(hexA);
  const b = hexToRgb(hexB);
  const clamped = Math.max(0, Math.min(1, weight));
  return rgbToHex({
    r: Math.round(a.r * clamped + b.r * (1 - clamped)),
    g: Math.round(a.g * clamped + b.g * (1 - clamped)),
    b: Math.round(a.b * clamped + b.b * (1 - clamped)),
  });
}

function alpha(hex: string, opacity: number) {
  const rgb = hexToRgb(hex);
  return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${Math.max(0, Math.min(1, opacity))})`;
}

function hexToRgb(hex: string) {
  const clean = hex.replace('#', '');
  const full = clean.length === 3 ? clean.split('').map((char) => char + char).join('') : clean.padEnd(6, '0').slice(0, 6);
  return {
    r: parseInt(full.slice(0, 2), 16),
    g: parseInt(full.slice(2, 4), 16),
    b: parseInt(full.slice(4, 6), 16),
  };
}

function rgbToHex(rgb: { r: number; g: number; b: number }) {
  return `#${toHex(rgb.r)}${toHex(rgb.g)}${toHex(rgb.b)}`;
}

function toHex(value: number) {
  return Math.max(0, Math.min(255, value)).toString(16).padStart(2, '0');
}
