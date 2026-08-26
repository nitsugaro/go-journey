import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { EditorView } from '@codemirror/view'
import { tags } from '@lezer/highlight'

const jsonHighlight = HighlightStyle.define([
  { tag: tags.propertyName, color: 'var(--color-code-property)', fontWeight: '700' },
  { tag: tags.string, color: 'var(--color-code-string)' },
  { tag: tags.number, color: 'var(--color-code-number)' },
  { tag: [tags.bool, tags.null, tags.atom], color: 'var(--color-code-atom)', fontWeight: '700' },
  { tag: tags.punctuation, color: 'var(--color-muted)' },
  { tag: tags.bracket, color: 'var(--color-muted)' },
])

const jsonTheme = EditorView.theme({
  '&': { backgroundColor: 'var(--color-surface-subtle)', color: 'var(--color-ink)' },
  '&.cm-focused': { backgroundColor: 'var(--color-surface-subtle)', outline: 'none' },
  '.cm-content': { caretColor: 'var(--color-ink)', padding: '12px 0' },
  '.cm-line': { padding: '0 14px' },
  '.cm-gutters': { backgroundColor: 'var(--color-surface-muted-transparent)', color: 'var(--color-muted-soft)', border: '0' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--color-surface-soft)' },
  '.cm-selectionBackground': { backgroundColor: 'var(--color-blue-selection) !important' },
  '.cm-cursor': { borderLeftColor: 'var(--color-ink)' },
})

export function SchemaJsonEditor({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <div className="flex min-h-[420px] flex-1 flex-col overflow-hidden rounded-3xl border border-[var(--color-border-subtle)] bg-[var(--color-surface-subtle)] shadow-inner">
      <div className="flex shrink-0 items-center justify-between border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] px-4 py-2">
        <div className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-red-border)]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-warning-border)]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-green-border)]" />
        </div>
        <span className="rounded-full bg-[var(--color-surface)] px-2.5 py-1 font-mono text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-muted-soft)] ring-1 ring-[var(--color-border-soft)]">
          JSON Schema
        </span>
      </div>
      <CodeMirror
        value={value}
        onChange={onChange}
        height="100%"
        basicSetup={{ lineNumbers: true, foldGutter: true, highlightActiveLine: true, highlightActiveLineGutter: true }}
        extensions={[json(), jsonTheme, syntaxHighlighting(jsonHighlight), EditorView.lineWrapping]}
      />
    </div>
  )
}
