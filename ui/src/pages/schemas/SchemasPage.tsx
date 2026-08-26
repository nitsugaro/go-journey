import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  deleteDeveloperSchema,
  getDeveloperSchema,
  listDeveloperSchemas,
  saveDeveloperSchema,
} from '../../api/journeyApi'
import type { DeveloperSchema } from '../../types/journey'
import { DeleteConfirmModal } from '../../components/DeleteConfirmModal'
import { SchemaEditorPanel } from './SchemaEditorPanel'
import { SchemaSidebar } from './SchemaSidebar'
import { newDeveloperSchema, schemaSearchText } from './schemaUtils'

export function SchemasPage() {
  const { realm = 'alpha', schemaId = '' } = useParams()
  const navigate = useNavigate()
  const [schemas, setSchemas] = useState<DeveloperSchema[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [draft, setDraft] = useState<DeveloperSchema>(() => newDeveloperSchema())
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [error, setError] = useState('')
  const [deleteTargetSchema, setDeleteTargetSchema] = useState<DeveloperSchema | null>(null)

  const filteredSchemas = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return schemas
    return schemas.filter((schema) => schemaSearchText(schema).includes(query))
  }, [schemas, search])

  useEffect(() => {
    void refreshSchemas()
  }, [realm])

  useEffect(() => {
    if (schemaId) {
      void selectSchema(schemaId, { navigate: false })
      return
    }
    startCreate(false)
  }, [realm, schemaId])

  async function refreshSchemas() {
    setLoading(true)
    setError('')
    try {
      const response = await listDeveloperSchemas(realm, { limit: 200 })
      setSchemas(response.result || [])
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  async function selectSchema(id: string, options: { navigate?: boolean } = {}) {
    if (!id) return
    if (options.navigate !== false && id !== schemaId) {
      navigate(`/${encodeURIComponent(realm)}/schemas/${encodeURIComponent(id)}`)
      return
    }
    setError('')
    try {
      const loaded = await getDeveloperSchema(realm, id)
      setSelectedID(loaded.id || '')
      setDraft(loaded)
      setDirty(false)
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  function startCreate(markDirty = true) {
    if (schemaId) navigate(`/${encodeURIComponent(realm)}/schemas`)
    setSelectedID('')
    setDraft(newDeveloperSchema())
    setDirty(markDirty)
    setError('')
  }

  function updateDraft(next: Partial<DeveloperSchema>) {
    setDraft((current) => ({ ...current, ...next }))
    setDirty(true)
  }

  async function persistSchema() {
    const name = draft.name.trim()
    if (!name) {
      setError('Schema name is required.')
      return
    }
    setSaving(true)
    setError('')
    try {
      const saved = await saveDeveloperSchema(realm, { ...draft, name, realm })
      setSelectedID(saved.id || '')
      setDraft(saved)
      setDirty(false)
      if (saved.id) navigate(`/${encodeURIComponent(realm)}/schemas/${encodeURIComponent(saved.id)}`, { replace: true })
      const response = await listDeveloperSchemas(realm, { limit: 200 })
      setSchemas(response.result || [])
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  async function confirmDeleteSchema() {
    const target = deleteTargetSchema
    if (!target?.id) return
    setSaving(true)
    setError('')
    try {
      await deleteDeveloperSchema(realm, target.id)
      const response = await listDeveloperSchemas(realm, { limit: 200 })
      setSchemas(response.result || [])
      setDeleteTargetSchema(null)
      const next = response.result?.[0]
      if (next?.id) navigate(`/${encodeURIComponent(realm)}/schemas/${encodeURIComponent(next.id)}`, { replace: true })
      else startCreate(false)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="flex h-full min-h-0 gap-4">
      <SchemaSidebar
        schemas={filteredSchemas}
        selectedID={selectedID}
        search={search}
        loading={loading}
        onSearch={setSearch}
        onCreate={() => startCreate()}
        onSelect={(schema) => selectSchema(schema.id || '')}
      />
      <SchemaEditorPanel
        draft={draft}
        selectedID={selectedID}
        dirty={dirty}
        saving={saving}
        error={error}
        onDraftChange={updateDraft}
        onSave={persistSchema}
        onDelete={() => selectedID && setDeleteTargetSchema({ ...draft, id: selectedID })}
      />
      <DeleteConfirmModal
        open={Boolean(deleteTargetSchema)}
        itemLabel="Schema"
        itemName={deleteTargetSchema?.name?.trim() || deleteTargetSchema?.id || ''}
        confirming={saving}
        onCancel={() => setDeleteTargetSchema(null)}
        onConfirm={confirmDeleteSchema}
      />
    </section>
  )
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Unexpected error'
}
