import { useEffect, useState } from 'react';
import { getScript } from '../../../api/journeyApi';
import type { ScriptArgument } from '../../../types/journey';
import { asRecord } from '../utils/schemaUtils';
import { schemaForScriptArgument } from '../utils/schemaUtils';
import { SchemaField } from './SchemaField';

export function ScriptArgsConfigField({
  realm,
  label,
  required,
  description,
  scriptID,
  value,
  onChange,
}: {
  realm: string;
  label: string;
  required?: boolean;
  description?: string;
  scriptID: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const [args, setArgs] = useState<ScriptArgument[]>([]);
  const [scriptName, setScriptName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const current = asRecord(value);

  useEffect(() => {
    let cancelled = false;
    setArgs([]);
    setScriptName('');
    setError('');
    if (!scriptID) return;

    setLoading(true);
    getScript(realm, scriptID)
      .then((script) => {
        if (cancelled) return;
        setScriptName(script.name || scriptID);
        setArgs(Array.isArray(script.args) ? script.args : []);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [realm, scriptID]);

  const setArg = (argID: string, nextValue: unknown) => {
    onChange({ ...current, [argID]: nextValue });
  };

  return (
    <div className="rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)] lg:col-span-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-muted-soft)]">
            {label}
            {required ? ' *' : ''}
          </p>
          <p className="mt-1 text-xs text-[var(--color-muted-soft)]">
            {description || 'Typed values passed to the selected script through the args binding.'}
          </p>
        </div>
        {scriptName && (
          <span className="max-w-44 truncate rounded-full bg-[var(--color-blue-soft)] px-3 py-1 text-xs font-semibold text-[var(--color-blue)]">
            {scriptName}
          </span>
        )}
      </div>

      <div className="mt-3 grid gap-3">
        {!scriptID && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            Select a script first to configure args.
          </p>
        )}
        {loading && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            Loading script args…
          </p>
        )}
        {error && (
          <p className="rounded-2xl bg-[var(--color-red-soft)] px-3 py-2 text-sm text-[var(--color-red)]">{error}</p>
        )}
        {!loading && !error && scriptID && args.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-muted-soft)]">
            This script does not declare args.
          </p>
        )}
        {args.map((arg) => (
          <SchemaField
            key={arg.id}
            realm={realm}
            name={arg.id}
            schema={schemaForScriptArgument(arg)}
            value={current[arg.id]}
            required
            onChange={(nextValue) => setArg(arg.id, nextValue)}
          />
        ))}
      </div>
    </div>
  );
}
