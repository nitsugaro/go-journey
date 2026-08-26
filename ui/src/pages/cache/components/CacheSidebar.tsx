import { IconButton } from '../../../components/Button';
import type { CacheInfo, CacheInstanceInfo } from '../../../types/journey';
import { formatBytes } from '../cacheUtils';
import { PlusIcon } from '../../../components/Icons';

type CacheSidebarProps = {
  caches: CacheInfo[];
  instances: CacheInstanceInfo[];
  filteredInstances: CacheInstanceInfo[];
  selectedKey: string;
  selectedID: string;
  cacheFilter: string;
  search: string;
  loading: boolean;
  onSearch: (value: string) => void;
  onCacheFilter: (value: string) => void;
  onCreate: () => void;
  onSelectCacheDraft: (key: string) => void;
  onSelectInstance: (cacheKey: string, instanceId: string) => void;
};

export function CacheSidebar(props: CacheSidebarProps) {
  return (
    <aside className="flex w-[min(420px,34vw)] shrink-0 flex-col overflow-hidden rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm">
      <SidebarHeader onCreate={props.onCreate} />
      <SidebarFilters
        caches={props.caches}
        search={props.search}
        cacheFilter={props.cacheFilter}
        onSearch={props.onSearch}
        onCacheFilter={props.onCacheFilter}
      />
      <InstanceList
        loading={props.loading}
        instances={props.filteredInstances}
        selectedKey={props.selectedKey}
        selectedID={props.selectedID}
        onSelect={props.onSelectInstance}
      />
    </aside>
  );
}

function SidebarHeader({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="border-b border-[var(--color-border-soft)] p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.32em] text-[var(--color-blue)]">Instances</p>
          <h1 className="mt-2 text-2xl font-bold tracking-tight text-[var(--color-ink)]">Instance catalog</h1>
          <p className="mt-1 text-sm text-[var(--color-muted)]">
            Create and edit configurable HTTP clients, LDAP pools, JWK caches and custom dependencies. Engine-managed
            runtime caches stay hidden.
          </p>
        </div>
        <IconButton onClick={onCreate} label="Create instance" variant="primary" size="lg">
          <PlusIcon />
        </IconButton>
      </div>
    </div>
  );
}

function SidebarFilters({
  caches,
  search,
  cacheFilter,
  onSearch,
  onCacheFilter,
}: {
  caches: CacheInfo[];
  search: string;
  cacheFilter: string;
  onSearch: (value: string) => void;
  onCacheFilter: (value: string) => void;
}) {
  return (
    <div className="border-b border-[var(--color-border-soft)] p-5">
      <div className="grid gap-3">
        <input
          value={search}
          onChange={(event) => onSearch(event.target.value)}
          placeholder="Search configured instance id or type..."
          className="w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
        />
        <select
          value={cacheFilter}
          onChange={(event) => onCacheFilter(event.target.value)}
          className="w-full rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] px-4 py-3 text-sm font-semibold text-[var(--color-ink)] outline-none ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]"
        >
          <option value="">All configurable types</option>
          {caches.map((cache) => (
            <option key={cache.key} value={cache.key}>
              {cache.key}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}

function InstanceList({
  loading,
  instances,
  selectedKey,
  selectedID,
  onSelect,
}: {
  loading: boolean;
  instances: CacheInstanceInfo[];
  selectedKey: string;
  selectedID: string;
  onSelect: (cacheKey: string, instanceID: string) => void;
}) {
  return (
    <div className="min-h-0 flex-1 overflow-auto p-3">
      {loading && (
        <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">
          Loading instances…
        </p>
      )}
      {!loading && instances.length === 0 && (
        <p className="rounded-2xl bg-[var(--color-surface-subtle)] px-4 py-3 text-sm text-[var(--color-muted)]">
          No configurable instances found.
        </p>
      )}
      <div className="grid gap-2">
        {instances.map((item) => (
          <InstanceCard
            key={`${item.cache_key}/${item.instance_id}`}
            item={item}
            selected={item.cache_key === selectedKey && item.instance_id === selectedID}
            onSelect={() => onSelect(item.cache_key, item.instance_id)}
          />
        ))}
      </div>
    </div>
  );
}

function InstanceCard({
  item,
  selected,
  onSelect,
}: {
  item: CacheInstanceInfo;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={[
        'rounded-3xl border p-4 text-left transition',
        selected
          ? 'border-[var(--color-blue)] bg-[var(--color-blue-subtle)] shadow-sm'
          : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] hover:border-[var(--color-blue-border)] hover:bg-[var(--color-surface-subtle)]',
      ].join(' ')}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-sm font-bold text-[var(--color-ink)]">{item.instance_id}</h2>
          <p className="mt-1 truncate font-mono text-xs font-semibold text-[var(--color-muted-soft)]">
            {item.cache_key}
          </p>
        </div>
        <span className="rounded-full bg-[var(--color-green-soft)] px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--color-green)] ring-1 ring-[var(--color-green-border)]">
          persisted
        </span>
      </div>
      <p className="mt-3 text-xs text-[var(--color-muted-soft)]">{formatBytes(item.size_bytes)}</p>
    </button>
  );
}
