import { useEffect, useMemo, useState } from 'react';
import { Button, IconButton } from '../components/Button';
import { brand, createPalette, defaultCustomPaletteSeed, type ThemePalette, type ThemePaletteSeed } from '../config/brand';

const selectedPaletteStorageKey = 'go-journey-studio.palette';
const customPalettesStorageKey = 'go-journey-studio.custom-palettes';
const customPrefix = 'custom:';

const paletteFields = [
  { key: 'bg', label: 'Background', helper: 'Main app background.' },
  { key: 'surface', label: 'Surface', helper: 'Cards, panels and nodes.' },
  { key: 'ink', label: 'Text', helper: 'Primary readable text.' },
  { key: 'accent', label: 'Accent', helper: 'Buttons, focus and selected states.' },
  { key: 'success', label: 'Success', helper: 'Positive/finished states.' },
  { key: 'danger', label: 'Danger', helper: 'Failure and destructive states.' },
] as const;

type StoredCustomPalette = {
  name: string;
  palette: ThemePalette;
};

type ThemePalettePickerProps = {
  onPaletteChange: (palette: ThemePalette) => void;
};

export function ThemePalettePicker({ onPaletteChange }: ThemePalettePickerProps) {
  const [open, setOpen] = useState(false);
  const [customOpen, setCustomOpen] = useState(false);
  const [customName, setCustomName] = useState('My palette');
  const [customSeed, setCustomSeed] = useState<ThemePaletteSeed>(defaultCustomPaletteSeed);
  const [customPalettes, setCustomPalettes] = useState<Record<string, StoredCustomPalette>>(() => readCustomPalettes());
  const [selectedPalette, setSelectedPalette] = useState(() => readSelectedPalette(customPalettes));
  const allPalettes = useMemo(
    () => ({
      ...brand.palettes,
      ...Object.fromEntries(Object.entries(customPalettes).map(([key, value]) => [`${customPrefix}${key}`, value.palette])),
    }) as Record<string, ThemePalette>,
    [customPalettes],
  );
  const activePalette = allPalettes[selectedPalette] ?? brand.palettes[brand.defaultPalette];
  const selectedLabel = getPaletteLabel(selectedPalette, customPalettes);

  useEffect(() => onPaletteChange(activePalette), [activePalette, onPaletteChange]);

  function selectPalette(name: string) {
    setSelectedPalette(name);
    localStorage.setItem(selectedPaletteStorageKey, name);
  }

  function saveCustomPalette() {
    const id = slugify(customName) || `palette-${Date.now()}`;
    const next = { ...customPalettes, [id]: { name: customName.trim() || 'Custom palette', palette: createPalette(customSeed) } };
    setCustomPalettes(next);
    localStorage.setItem(customPalettesStorageKey, JSON.stringify(next));
    selectPalette(`${customPrefix}${id}`);
    setCustomOpen(false);
  }

  function removeCustomPalette(id: string) {
    const next = { ...customPalettes };
    delete next[id];
    setCustomPalettes(next);
    localStorage.setItem(customPalettesStorageKey, JSON.stringify(next));
    if (selectedPalette === `${customPrefix}${id}`) selectPalette(brand.defaultPalette);
  }

  return (
    <div className="relative">
      <Button onClick={() => setOpen((value) => !value)} variant="secondary">
        <span className="h-3 w-3 rounded-full bg-[var(--color-blue)] shadow-sm ring-2 ring-[var(--color-blue-soft)]" />
        {selectedLabel}
      </Button>
      {open && (
        <div className="motion-popover-panel absolute right-0 top-14 z-50 w-[min(520px,calc(100vw-2rem))] overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface-overlay)] shadow-2xl shadow-[var(--color-muted-faint)] backdrop-blur">
          <ThemeMenuHeader onClose={() => setOpen(false)} />
          <div className="max-h-[70vh] overflow-auto p-4">
            <div className="grid gap-2 sm:grid-cols-2">
              {Object.entries(brand.palettes).map(([name, palette]) => (
                <PaletteButton key={name} name={name} label={brand.paletteLabels[name as keyof typeof brand.paletteLabels]} palette={palette} selected={selectedPalette === name} onClick={() => selectPalette(name)} />
              ))}
              {Object.entries(customPalettes).map(([name, item]) => (
                <PaletteButton key={name} name={`${customPrefix}${name}`} label={item.name} palette={item.palette} selected={selectedPalette === `${customPrefix}${name}`} onClick={() => selectPalette(`${customPrefix}${name}`)} onDelete={() => removeCustomPalette(name)} custom />
              ))}
            </div>
            <CustomPaletteForm open={customOpen} name={customName} seed={customSeed} onToggle={() => setCustomOpen((value) => !value)} onNameChange={setCustomName} onSeedChange={setCustomSeed} onCancel={() => setCustomOpen(false)} onSave={saveCustomPalette} />
          </div>
        </div>
      )}
    </div>
  );
}

function ThemeMenuHeader({ onClose }: { onClose: () => void }) {
  return (
    <div className="border-b border-[var(--color-border-soft)] p-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Theme</p>
          <h2 className="mt-1 text-lg font-bold text-[var(--color-ink)]">Choose palette</h2>
          <p className="mt-1 text-sm text-[var(--color-muted)]">12 built-in enterprise palettes. Custom palettes derive the rest from 6 base colors.</p>
        </div>
        <Button onClick={onClose} variant="secondary" size="sm">Close</Button>
      </div>
    </div>
  );
}

function CustomPaletteForm({ open, name, seed, onToggle, onNameChange, onSeedChange, onCancel, onSave }: {
  open: boolean;
  name: string;
  seed: ThemePaletteSeed;
  onToggle: () => void;
  onNameChange: (name: string) => void;
  onSeedChange: (seed: ThemePaletteSeed) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  return (
    <div className="mt-4 rounded-3xl border border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] p-4">
      <button type="button" onClick={onToggle} className="flex w-full items-center justify-between rounded-xl px-1 py-1 text-left text-sm font-bold text-[var(--color-ink)] transition hover:bg-[var(--color-surface-soft)]">
        <span>Create custom palette</span>
        <span className="rounded-lg bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-blue)] ring-1 ring-[var(--color-blue-border)]">6 colors</span>
      </button>
      {open && (
        <div className="mt-4 grid gap-4">
          <label className="grid gap-1">
            <span className="text-xs font-bold uppercase tracking-[0.22em] text-[var(--color-muted-soft)]">Name</span>
            <input value={name} onChange={(event) => onNameChange(event.target.value)} className="rounded-2xl border border-transparent bg-[var(--color-surface)] px-3 py-2 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]" />
          </label>
          <div className="grid gap-3 sm:grid-cols-2">
            {paletteFields.map((field) => (
              <label key={field.key} className="flex items-center gap-3 rounded-2xl bg-[var(--color-surface)] p-3 ring-1 ring-[var(--color-border-soft)]">
                <input type="color" value={seed[field.key]} onChange={(event) => onSeedChange({ ...seed, [field.key]: event.target.value })} className="h-10 w-12 cursor-pointer rounded-xl border-0 bg-transparent p-0" />
                <span className="min-w-0"><span className="block text-sm font-bold text-[var(--color-ink)]">{field.label}</span><span className="block truncate text-xs text-[var(--color-muted)]">{field.helper}</span></span>
              </label>
            ))}
          </div>
          <div className="flex justify-end gap-2"><Button onClick={onCancel} variant="ghost">Cancel</Button><Button onClick={onSave} variant="primary">Save palette</Button></div>
        </div>
      )}
    </div>
  );
}

function PaletteButton({ name, label, palette, selected, custom, onClick, onDelete }: { name: string; label: string; palette: ThemePalette; selected: boolean; custom?: boolean; onClick: () => void; onDelete?: () => void }) {
  return (
    <button type="button" onClick={onClick} className={['rounded-2xl border p-3 text-left transition', selected ? 'border-[var(--color-blue)] bg-[var(--color-blue-subtle)] shadow-sm' : 'border-[var(--color-border-soft)] bg-[var(--color-surface)] hover:border-[var(--color-blue-border)] hover:bg-[var(--color-surface-muted-transparent)]'].join(' ')}>
      <span className="flex items-center justify-between gap-3">
        <span><span className="block text-sm font-bold text-[var(--color-ink)]">{label}</span><span className="block text-xs text-[var(--color-muted)]">{custom ? 'Custom' : name}</span></span>
        <span className="flex items-center gap-2">{selected && <span className="rounded-lg bg-[var(--color-blue)] px-2 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--color-white)]">Active</span>}{onDelete && <IconButton onClick={(event) => { event.stopPropagation(); onDelete(); }} label="Delete custom palette" variant="danger" size="xs">×</IconButton>}</span>
      </span>
      <span className="mt-3 flex overflow-hidden rounded-full ring-1 ring-[var(--color-border-soft)]">{[palette.bg, palette.surface, palette.blue, palette.green, palette.warning, palette.red, palette.ink].map((color, index) => <span key={`${color}-${index}`} className="h-4 flex-1" style={{ backgroundColor: color }} />)}</span>
    </button>
  );
}

function getPaletteLabel(selected: string, customPalettes: Record<string, StoredCustomPalette>) {
  if (selected.startsWith(customPrefix)) return customPalettes[selected.slice(customPrefix.length)]?.name ?? 'Custom';
  return brand.paletteLabels[selected as keyof typeof brand.paletteLabels] ?? brand.paletteLabels[brand.defaultPalette];
}

function readSelectedPalette(customPalettes: Record<string, StoredCustomPalette>) {
  const stored = localStorage.getItem(selectedPaletteStorageKey);
  if (stored && (stored in brand.palettes || (stored.startsWith(customPrefix) && stored.slice(customPrefix.length) in customPalettes))) return stored;
  return brand.defaultPalette;
}

function readCustomPalettes() {
  try {
    const parsed = JSON.parse(localStorage.getItem(customPalettesStorageKey) || '{}') as Record<string, StoredCustomPalette>;
    return Object.fromEntries(Object.entries(parsed).filter(([, value]) => value && typeof value.name === 'string' && isThemePalette(value.palette)));
  } catch {
    return {};
  }
}

function isThemePalette(value: unknown): value is ThemePalette {
  if (!value || typeof value !== 'object') return false;
  const palette = value as Record<string, unknown>;
  return Object.keys(brand.palettes.default).every((key) => typeof palette[key] === 'string');
}

function slugify(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
}
