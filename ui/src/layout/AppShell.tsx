import { useCallback, useMemo, useState } from 'react';
import { NavLink, Outlet, useParams } from 'react-router-dom';
import { brand, type ThemePalette } from '../config/brand';
import { ThemePalettePicker } from './ThemePalettePicker';

const navItems = [
  { path: 'flow', label: 'Flow' },
  { path: 'scripts', label: 'Scripts' },
  { path: 'schemas', label: 'Schemas' },
  { path: 'schedules', label: 'Schedules' },
  { path: 'instances', label: 'Instances' },
];

export function AppShell() {
  const { realm = 'alpha' } = useParams();
  const [activePalette, setActivePalette] = useState<ThemePalette>(brand.palettes[brand.defaultPalette]);
  const cssVars = useMemo(
    () => Object.fromEntries(Object.entries(activePalette).map(([name, value]) => [`--color-${toKebab(name)}`, value])) as React.CSSProperties,
    [activePalette],
  );
  const applyPalette = useCallback((palette: ThemePalette) => {
    setActivePalette(palette);
    for (const [name, value] of Object.entries(palette)) {
      document.documentElement.style.setProperty(`--color-${toKebab(name)}`, value);
    }
  }, []);

  return (
    <div style={cssVars} className="flex h-screen flex-col overflow-hidden bg-[var(--color-bg)] text-[var(--color-ink)]">
      <header className="shrink-0 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
        <div className="flex items-center justify-between px-5 py-3">
          <div>
            <p className="text-lg font-semibold tracking-tight text-[var(--color-ink)]">{brand.appName}</p>
            <p className="text-sm text-[var(--color-muted)]">{brand.tagline}</p>
          </div>
          <div className="flex items-center gap-3">
            <ThemePalettePicker onPaletteChange={applyPalette} />
            <nav className="flex rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-1">
              {navItems.map((item) => (
                <NavLink key={item.path} to={`/${encodeURIComponent(realm)}/${item.path}`} className={({ isActive }) => ['rounded-xl px-4 py-2 text-sm font-medium transition', isActive ? 'bg-[var(--color-blue)] text-[var(--color-white)] shadow-sm' : 'text-[var(--color-muted)] hover:bg-[var(--color-surface)] hover:text-[var(--color-blue)]'].join(' ')}>
                  {item.label}
                </NavLink>
              ))}
            </nav>
          </div>
        </div>
      </header>
      <main className="min-h-0 flex-1 overflow-hidden px-4 py-4">
        <Outlet />
      </main>
    </div>
  );
}

function toKebab(value: string) {
  return value.replace(/[A-Z]/g, (match) => `-${match.toLowerCase()}`);
}
