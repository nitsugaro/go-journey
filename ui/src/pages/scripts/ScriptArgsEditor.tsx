import { IconButton } from '../../components/Button';
import { TagsInput } from '../../components/TagsInput';
import type { ScriptArgument } from '../../types/journey';
import { DeleteIcon, PlusIcon } from '../../components/Icons';

const scriptArgumentTypes = ['string', 'bool', 'int', 'float', 'list', 'object'] as const;

export function ScriptArgsEditor({
  args,
  onChange,
}: {
  args: ScriptArgument[];
  onChange: (args: ScriptArgument[]) => void;
}) {
  const rows = Array.isArray(args) ? args : [];
  const updateArg = (index: number, next: ScriptArgument) =>
    onChange(rows.map((row, rowIndex) => (rowIndex === index ? normalizeScriptArgument(next) : row)));
  const removeArg = (index: number) => onChange(rows.filter((_, rowIndex) => rowIndex !== index));
  const addArg = () => onChange([...rows, { id: nextArgID(rows), type: 'string' }]);

  return (
    <section className="grid gap-3">
      <div className="flex items-start justify-between gap-3 rounded-3xl bg-[var(--color-surface-subtle)] p-4 ring-1 ring-[var(--color-border-soft)]">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.24em] text-[var(--color-muted-soft)]">Args schema</p>
          <p className="mt-1 text-sm text-[var(--color-muted)]">
            Typed values expected by this script. Script steps use this to render their args form.
          </p>
        </div>
        <IconButton onClick={addArg} label="Add script argument" variant="secondary" size="sm">
          <PlusIcon />
        </IconButton>
      </div>
      <div className="grid gap-3">
        {rows.length === 0 && (
          <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted-soft)] ring-1 ring-[var(--color-border-soft)]">
            No args declared.
          </p>
        )}
        {rows.map((arg, index) => (
          <ScriptArgRow
            key={`script-arg-${index}`}
            arg={arg}
            onChange={(next) => updateArg(index, next)}
            onRemove={() => removeArg(index)}
          />
        ))}
      </div>
    </section>
  );
}

function ScriptArgRow({
  arg,
  onChange,
  onRemove,
}: {
  arg: ScriptArgument;
  onChange: (arg: ScriptArgument) => void;
  onRemove: () => void;
}) {
  const type = normalizeScriptArgumentType(arg.type);
  return (
    <div className="min-w-0 rounded-2xl bg-[var(--color-surface-subtle)] p-3 ring-1 ring-[var(--color-border-soft)]">
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_120px_auto] items-center gap-2">
        <input
          value={arg.id}
          onChange={(event) => onChange({ ...arg, id: event.target.value })}
          placeholder="arg id"
          className="min-w-0 rounded-xl border border-transparent bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]"
        />
        <select
          value={type}
          onChange={(event) =>
            onChange({ ...arg, type: event.target.value, enum: event.target.value === 'string' ? arg.enum : undefined })
          }
          className="min-w-0 rounded-xl border border-transparent bg-[var(--color-surface)] px-3 py-2.5 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:ring-2 focus:ring-[var(--color-blue)]"
        >
          {scriptArgumentTypes.map((item) => (
            <option key={item} value={item}>
              {item}
            </option>
          ))}
        </select>
        <IconButton onClick={onRemove} label="Remove script argument" variant="ghost" size="sm">
          <DeleteIcon />
        </IconButton>
      </div>
      {type === 'string' && (
        <div className="mt-2">
          <TagsInput
            value={arg.enum || []}
            onChange={(values) => onChange({ ...arg, enum: values })}
            placeholder="Optional enum values…"
            className="min-h-[40px] rounded-xl bg-[var(--color-surface)] p-1.5 ring-1 ring-[var(--color-border-soft)]"
          />
        </div>
      )}
    </div>
  );
}

function normalizeScriptArgument(arg: ScriptArgument): ScriptArgument {
  const clean: ScriptArgument = { id: arg.id.trim(), type: normalizeScriptArgumentType(arg.type) };
  const enumValues = (arg.enum || []).map((value) => value.trim()).filter(Boolean);
  if (clean.type === 'string' && enumValues.length > 0) clean.enum = Array.from(new Set(enumValues));
  return clean;
}

function normalizeScriptArgumentType(type: string | undefined) {
  return scriptArgumentTypes.includes(type as (typeof scriptArgumentTypes)[number]) ? type || 'string' : 'string';
}

function nextArgID(rows: ScriptArgument[]) {
  let index = rows.length + 1;
  let id = `arg_${index}`;
  const used = new Set(rows.map((row) => row.id));
  while (used.has(id)) {
    index += 1;
    id = `arg_${index}`;
  }
  return id;
}
