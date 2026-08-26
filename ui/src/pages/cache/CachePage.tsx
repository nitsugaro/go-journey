import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { deleteInstance, getInstance, listInstances, saveInstance } from '../../api/journeyApi';
import { DeleteConfirmModal } from '../../components/DeleteConfirmModal';
import type { CacheInfo, CacheInstanceInfo } from '../../types/journey';
import { CacheEditorPanel } from './components/CacheEditorPanel';
import { CacheSidebar } from './components/CacheSidebar';
import { defaultConfigText, errorMessage, isGeneratedTemplate, stringifyConfig } from './cacheUtils';

export function CachePage() {
  const { realm = 'alpha' } = useParams();
  const [caches, setCaches] = useState<CacheInfo[]>([]);
  const [instances, setInstances] = useState<CacheInstanceInfo[]>([]);
  const [selectedKey, setSelectedKey] = useState('');
  const [selectedID, setSelectedID] = useState('');
  const [search, setSearch] = useState('');
  const [cacheFilter, setCacheFilter] = useState('');
  const [draftCacheKey, setDraftCacheKey] = useState('');
  const [draftInstanceID, setDraftInstanceID] = useState('');
  const [configText, setConfigText] = useState('{}');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [deleteTargetInstance, setDeleteTargetInstance] = useState<{ cacheKey: string; instanceID: string; name: string } | null>(null);

  const editableCaches = useMemo(() => caches.filter(isUserConfigurableCache), [caches]);
  const editableCacheKeys = useMemo(() => new Set(editableCaches.map((item) => item.key)), [editableCaches]);
  const editableInstances = useMemo(() => instances.filter((item) => item.persisted && editableCacheKeys.has(item.cache_key)), [editableCacheKeys, instances]);
  const selectedInstance = useMemo(() => editableInstances.find((item) => item.cache_key === selectedKey && item.instance_id === selectedID), [editableInstances, selectedID, selectedKey]);
  const selectedCache = useMemo(() => editableCaches.find((item) => item.key === draftCacheKey) || editableCaches.find((item) => item.key === selectedKey), [draftCacheKey, editableCaches, selectedKey]);
  const filteredInstances = useMemo(() => filterInstances(editableInstances, cacheFilter, search), [cacheFilter, editableInstances, search]);
  const isNew = !selectedInstance;

  useEffect(() => { void refreshInstances(); }, [realm]);

  async function refreshInstances(select?: { cache_key: string; instance_id: string }) {
    setLoading(true);
    setError('');
    try {
      const response = await listInstances(realm, { limit: 500 });
      const nextCaches = (response.caches || []).filter(isUserConfigurableCache);
      const nextCacheKeys = new Set(nextCaches.map((item) => item.key));
      const nextInstances = (response.result || []).filter((item) => item.persisted && nextCacheKeys.has(item.cache_key));
      setCaches(nextCaches);
      setInstances(nextInstances);
      const next = select || nextInstances[0];
      if (next) await selectInstance(next.cache_key, next.instance_id);
      else startCreate(nextCaches[0]?.key || '');
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  async function selectInstance(cacheKey: string, instanceID: string) {
    if (!cacheKey || !instanceID) return;
    setError('');
    try {
      const item = await getInstance(realm, cacheKey, instanceID);
      setSelectedKey(item.cache_key);
      setSelectedID(item.instance_id);
      setDraftCacheKey(item.cache_key);
      setDraftInstanceID(item.instance_id);
      setConfigText(stringifyConfig(item.config));
      setDirty(false);
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  function startCreate(cacheKey = cacheFilter || editableCaches[0]?.key || '') {
    const cache = editableCaches.find((item) => item.key === cacheKey);
    setSelectedKey('');
    setSelectedID('');
    setDraftCacheKey(cacheKey);
    setDraftInstanceID('');
    setConfigText(defaultConfigText(cache));
    setDirty(true);
    setError('');
  }

  function updateConfig(next: string) {
    setConfigText(next);
    setDirty(true);
  }

  function selectCacheDraft(cacheKey: string) {
    const shouldSwapTemplate = isNew && isGeneratedTemplate(configText, editableCaches);
    setDraftCacheKey(cacheKey);
    if (shouldSwapTemplate) setConfigText(defaultConfigText(editableCaches.find((item) => item.key === cacheKey)));
    setDirty(true);
  }

  async function persistInstance() {
    const cacheKey = draftCacheKey.trim();
    const instanceID = draftInstanceID.trim();
    if (!cacheKey || !instanceID) return setError('Cache key and instance id are required.');
    let parsed: unknown;
    try {
      parsed = JSON.parse(configText || '{}');
    } catch (err) {
      return setError(`Invalid JSON config: ${errorMessage(err)}`);
    }

    setSaving(true);
    setError('');
    try {
      const saved = await saveInstance(realm, cacheKey, instanceID, parsed);
      applySavedInstance(saved);
      await refreshInstances({ cache_key: saved.cache_key, instance_id: saved.instance_id });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function confirmDeleteInstance() {
    const target = deleteTargetInstance;
    if (!target) return;
    setSaving(true);
    setError('');
    try {
      await deleteInstance(realm, target.cacheKey, target.instanceID);
      setDeleteTargetInstance(null);
      setSelectedKey('');
      setSelectedID('');
      await refreshInstances();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  function applySavedInstance(saved: CacheInstanceInfo) {
    setSelectedKey(saved.cache_key);
    setSelectedID(saved.instance_id);
    setDraftCacheKey(saved.cache_key);
    setDraftInstanceID(saved.instance_id);
    setConfigText(stringifyConfig(saved.config));
    setDirty(false);
  }

  return (
    <section className="flex h-full min-h-0 gap-4">
      <CacheSidebar caches={editableCaches} instances={editableInstances} filteredInstances={filteredInstances} selectedKey={selectedKey} selectedID={selectedID} cacheFilter={cacheFilter} search={search} loading={loading} onSearch={setSearch} onCacheFilter={setCacheFilter} onCreate={() => startCreate()} onSelectCacheDraft={selectCacheDraft} onSelectInstance={selectInstance} />
      <CacheEditorPanel realm={realm} caches={editableCaches} selectedCache={selectedCache} selectedID={selectedID} draftCacheKey={draftCacheKey} draftInstanceID={draftInstanceID} configText={configText} dirty={dirty} saving={saving} error={error} isNew={isNew} onDraftCacheKey={selectCacheDraft} onDraftInstanceID={setDraftInstanceID} onConfigText={updateConfig} onDirty={() => setDirty(true)} onDelete={() => selectedKey && selectedID && setDeleteTargetInstance({ cacheKey: selectedKey, instanceID: selectedID, name: `${selectedKey}/${selectedID}` })} onSave={persistInstance} />
      <DeleteConfirmModal
        open={Boolean(deleteTargetInstance)}
        itemLabel="Instance"
        itemName={deleteTargetInstance?.name || ''}
        confirming={saving}
        onCancel={() => setDeleteTargetInstance(null)}
        onConfirm={confirmDeleteInstance}
      />
    </section>
  );
}

function isUserConfigurableCache(cache: CacheInfo) {
  return cache.user_configurable === true;
}

function filterInstances(instances: CacheInstanceInfo[], cacheFilter: string, search: string) {
  const query = search.trim().toLowerCase();
  return instances.filter((item) => {
    if (cacheFilter && item.cache_key !== cacheFilter) return false;
    if (!query) return true;
    return `${item.cache_key} ${item.instance_id}`.toLowerCase().includes(query);
  });
}
