import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  deleteScript,
  getScript,
  listScriptBindings,
  listScripts,
  saveScript,
} from '../../api/journeyApi';
import { IconButton } from '../../components/Button';
import { DeleteConfirmModal } from '../../components/DeleteConfirmModal';
import { normalizeScriptType, scriptTypeOptionsFromBindingSets } from '../../config/typeOptions';
import type { JourneyScript, ScriptBindingSet } from '../../types/journey';
import {
  decodeBase64,
  defaultCode,
  encodeBase64,
  errorMessage,
  newScript,
  ScriptEditorPanel,
} from './ScriptEditorPanel';
import { PlusIcon, SavedIcon } from '../../components/Icons';

export function ScriptsPage() {
  const { realm = 'alpha', scriptId = '' } = useParams();
  const navigate = useNavigate();
  const [scripts, setScripts] = useState<JourneyScript[]>([]);
  const [bindingSets, setBindingSets] = useState<ScriptBindingSet[]>([]);
  const [selectedID, setSelectedID] = useState('');
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [draft, setDraft] = useState<JourneyScript>(() => newScript());
  const [code, setCode] = useState(defaultCode('auth'));
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [dirty, setDirty] = useState(false);
  const [deleteTargetScript, setDeleteTargetScript] = useState<JourneyScript | null>(null);
  const scriptOptions = useMemo(() => scriptTypeOptionsFromBindingSets(bindingSets), [bindingSets]);
  const scriptTypes = useMemo(() => scriptOptions.map((option) => option.value), [scriptOptions]);

  const filteredScripts = useMemo(() => {
    const query = search.trim().toLowerCase();
    return scripts.filter((script) => {
      if (typeFilter && normalizeScriptType(script.type) !== typeFilter) return false;
      if (!query) return true;
      return `${script.name || ''} ${script.type || ''} ${script.id || ''}`.toLowerCase().includes(query);
    });
  }, [scripts, search, typeFilter]);

  useEffect(() => {
    void refreshScripts();
    void refreshScriptTypes();
  }, [realm]);

  useEffect(() => {
    if (scriptId) {
      void selectScript(scriptId, { navigate: false });
      return;
    }
    setSelectedID('');
    setDraft(newScript());
    setCode(defaultCode('auth'));
    setDirty(false);
    setError('');
  }, [realm, scriptId]);

  async function refreshScripts() {
    setLoading(true);
    setError('');
    try {
      const response = await listScripts(realm, { limit: 200 });
      setScripts(response.result || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  async function refreshScriptTypes() {
    try {
      const response = await listScriptBindings(realm);
      setBindingSets(response.result || []);
    } catch {
      setBindingSets([]);
    }
  }

  async function selectScript(scriptID: string, options: { navigate?: boolean } = {}) {
    if (!scriptID) return;
    if (options.navigate !== false && scriptID !== scriptId) {
      navigate(`/${encodeURIComponent(realm)}/scripts/${encodeURIComponent(scriptID)}`);
      return;
    }
    setError('');
    try {
      const script = await getScript(realm, scriptID);
      setSelectedID(script.id || '');
      setDraft({ ...script, type: normalizeScriptType(script.type) });
      setCode(decodeBase64(script.code_base64));
      setDirty(false);
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  function startCreate() {
    const next = newScript();
    if (scriptId) navigate(`/${encodeURIComponent(realm)}/scripts`);
    setSelectedID('');
    setDraft(next);
    setCode(defaultCode(next.type));
    setDirty(true);
    setError('');
  }

  function updateDraft(next: Partial<JourneyScript>) {
    setDraft((current) => {
      const updated = { ...current, ...next };
      if (next.type && next.type !== current.type && !dirty && !current.id) {
        setCode(defaultCode(next.type));
      }
      return updated;
    });
    setDirty(true);
  }

  async function persistScript() {
    const name = draft.name.trim();
    if (!name) {
      setError('Script name is required.');
      return;
    }
    if (!code.trim()) {
      setError('Script code is required.');
      return;
    }

    setSaving(true);
    setError('');
    try {
      const saved = await saveScript(realm, {
        ...draft,
        name,
        type: normalizeScriptType(draft.type),
        code_base64: encodeBase64(code),
      });
      setSelectedID(saved.id || '');
      setDraft({ ...saved, type: normalizeScriptType(saved.type) });
      setCode(decodeBase64(saved.code_base64));
      setDirty(false);
      if (saved.id) {
        navigate(`/${encodeURIComponent(realm)}/scripts/${encodeURIComponent(saved.id)}`, { replace: true });
      }
      const response = await listScripts(realm, { limit: 200 });
      setScripts(response.result || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function confirmDeleteScript() {
    const target = deleteTargetScript;
    if (!target?.id) return;

    setSaving(true);
    setError('');
    try {
      await deleteScript(realm, target.id);
      const response = await listScripts(realm, { limit: 200 });
      setScripts(response.result || []);
      const next = response.result?.[0];
      setDeleteTargetScript(null);
      if (next?.id) {
        navigate(`/${encodeURIComponent(realm)}/scripts/${encodeURIComponent(next.id)}`, { replace: true });
      } else {
        navigate(`/${encodeURIComponent(realm)}/scripts`, { replace: true });
        startCreate();
        setDirty(false);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="flex h-full min-h-0 gap-4">
      <aside className="flex w-[min(420px,34vw)] shrink-0 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
        <div className="border-b border-[var(--color-border-soft)] p-5">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Scripts</p>
              <h1 className="mt-2 text-2xl font-bold tracking-tight text-[var(--color-ink)]">Script library</h1>
              <p className="mt-1 text-sm text-[var(--color-muted)]">
                Search, create, delete and edit JavaScript scripts.
              </p>
            </div>
            <IconButton onClick={startCreate} label="Create script" variant="primary" size="lg">
              <PlusIcon />
            </IconButton>
          </div>
          <div className="mt-4 grid gap-3">
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by name, type or UUID..."
              className="w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
            />
            <select
              value={typeFilter}
              onChange={(event) => setTypeFilter(event.target.value)}
              className="w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
            >
              <option value="">All script types</option>
              {scriptTypes.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-auto p-3">
          {loading && (
            <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">
              Loading scripts…
            </p>
          )}
          {!loading && filteredScripts.length === 0 && (
            <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">
              No scripts found.
            </p>
          )}
          <div className="grid gap-2">
            {filteredScripts.map((script) => {
              const selected = selectedID === script.id;
              return (
                <button
                  key={script.id || script.name}
                  type="button"
                  onClick={() => selectScript(script.id || '')}
                  className={[
                    'rounded-3xl border p-4 text-left transition',
                    selected
                      ? 'border-[var(--color-blue)] bg-[var(--color-blue-subtle)] shadow-sm'
                      : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] hover:border-[var(--color-blue-border)] hover:bg-[var(--color-surface-subtle)]',
                  ].join(' ')}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h2 className="truncate text-sm font-bold text-[var(--color-ink)]">
                        {script.name || 'Unnamed script'}
                      </h2>
                      <p className="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-muted-soft)]">
                        {normalizeScriptType(script.type)}
                      </p>
                    </div>
                    {selected && (
                      <span
                        title="Selected"
                        aria-label="Selected"
                        className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-[var(--color-blue)] text-xs font-bold text-[var(--color-white)]"
                      >
                        <SavedIcon size={14} />
                      </span>
                    )}
                  </div>
                  {script.id && (
                    <p className="mt-3 truncate font-mono text-[11px] text-[var(--color-muted-soft)]">{script.id}</p>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      </aside>

      <ScriptEditorPanel
        realm={realm}
        draft={draft}
        code={code}
        selectedID={selectedID}
        dirty={dirty}
        saving={saving}
        error={error}
        scriptTypeOptions={scriptOptions}
        bindingSets={bindingSets}
        onDraftChange={updateDraft}
        onCodeChange={(next) => {
          setCode(next);
          setDirty(true);
        }}
        onSave={persistScript}
        onDelete={() => selectedID && setDeleteTargetScript({ ...draft, id: selectedID })}
      />
      <DeleteConfirmModal
        open={Boolean(deleteTargetScript)}
        itemLabel="Script"
        itemName={deleteTargetScript?.name?.trim() || deleteTargetScript?.id || ''}
        confirming={saving}
        onCancel={() => setDeleteTargetScript(null)}
        onConfirm={confirmDeleteScript}
      />
    </section>
  );
}
