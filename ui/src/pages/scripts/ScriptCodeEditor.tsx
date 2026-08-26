import CodeMirror from '@uiw/react-codemirror';
import { autocompletion, type Completion, type CompletionContext } from '@codemirror/autocomplete';
import { javascript } from '@codemirror/lang-javascript';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { EditorView } from '@codemirror/view';
import { tags } from '@lezer/highlight';
import { useMemo } from 'react';
import type { ScriptBindingDescriptor } from '../../types/journey';

const scriptEditorHighlight = HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--color-code-keyword)', fontWeight: '700' },
  { tag: [tags.atom, tags.bool, tags.null], color: 'var(--color-code-atom)', fontWeight: '700' },
  { tag: tags.number, color: 'var(--color-code-number)' },
  { tag: tags.string, color: 'var(--color-code-string)' },
  { tag: tags.variableName, color: 'var(--color-code-variable)' },
  { tag: tags.definition(tags.variableName), color: 'var(--color-blue)', fontWeight: '700' },
  { tag: tags.propertyName, color: 'var(--color-code-property)', fontWeight: '650' },
  { tag: tags.operator, color: 'var(--color-code-operator)', fontWeight: '700' },
  { tag: tags.comment, color: 'var(--color-muted-soft)', fontStyle: 'italic' },
  { tag: tags.punctuation, color: 'var(--color-muted)' },
  { tag: tags.bracket, color: 'var(--color-muted)' },
]);

const scriptEditorTheme = EditorView.theme({
  '&': { backgroundColor: 'var(--color-surface-subtle)', color: 'var(--color-ink)' },
  '&.cm-focused': { backgroundColor: 'var(--color-surface-subtle)', outline: 'none' },
  '.cm-content': { caretColor: 'var(--color-ink)', padding: '10px 0' },
  '.cm-line': { padding: '0 14px' },
  '.cm-gutters': {
    backgroundColor: 'var(--color-surface-muted-transparent)',
    color: 'var(--color-muted-soft)',
    border: '0',
  },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--color-surface-soft)' },
  '.cm-selectionBackground': { backgroundColor: 'var(--color-blue-selection) !important' },
  '.cm-cursor': { borderLeftColor: 'var(--color-ink)' },
  '.cm-matchingBracket': {
    backgroundColor: 'var(--color-blue-subtle)',
    color: 'var(--color-ink)',
    outline: '1px solid var(--color-blue-border)',
  },
});

export function ScriptCodeEditor({
  code,
  bindings,
  onChange,
}: {
  code: string;
  bindings: ScriptBindingDescriptor[];
  onChange: (code: string) => void;
}) {
  const completions = useMemo(() => flattenBindingCompletions(bindings), [bindings]);
  const autocomplete = useMemo(() => scriptBindingsAutocomplete(completions), [completions]);

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-3xl border border-[var(--color-border-subtle)] bg-[var(--color-surface-subtle)] shadow-inner">
      <div className="flex shrink-0 items-center justify-between border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted-transparent)] px-4 py-2">
        <div className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-red-border)]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-warning-border)]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-green-border)]" />
        </div>
        <span className="rounded-full bg-[var(--color-surface)] px-2.5 py-1 font-mono text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-muted-soft)] ring-1 ring-[var(--color-border-soft)]">
          JavaScript
        </span>
      </div>
      <CodeMirror
        value={code}
        onChange={onChange}
        height="100%"
        basicSetup={{
          lineNumbers: true,
          foldGutter: true,
          highlightActiveLine: true,
          highlightActiveLineGutter: true,
          autocompletion: true,
        }}
        extensions={[
          javascript({ jsx: false, typescript: false }),
          autocomplete,
          syntaxHighlighting(scriptEditorHighlight),
          scriptEditorTheme,
          EditorView.lineWrapping,
        ]}
        className="journey-script-editor min-h-0 flex-1 overflow-hidden text-sm"
      />
    </div>
  );
}

function flattenBindingCompletions(bindings: ScriptBindingDescriptor[]) {
  const completions: Completion[] = [];
  const visit = (descriptor: ScriptBindingDescriptor, prefix = '') => {
    if (!descriptor.name) return;
    const fullName = prefix ? `${prefix}.${descriptor.name}` : descriptor.name;
    const isFunction = descriptor.type === 'function';
    completions.push({
      label: fullName,
      type: completionType(descriptor.type),
      detail: descriptor.signature || descriptor.type,
      info: descriptor.description || descriptor.example || descriptor.signature,
      apply: descriptor.example || (isFunction ? `${fullName}()` : fullName),
      boost: prefix ? 70 : 90,
    });
    descriptor.children?.forEach((child) => visit(child, fullName));
  };
  bindings.forEach((binding) => visit(binding));
  return completions;
}

function scriptBindingsAutocomplete(completions: Completion[]) {
  return autocompletion({
    activateOnTyping: true,
    override: [
      (context: CompletionContext) => {
        const word = context.matchBefore(/[\w.$]*/);
        if (!word || (word.from === word.to && !context.explicit)) return null;
        return { from: word.from, options: completions, validFor: /^[\w.$]*$/ };
      },
    ],
  });
}

function completionType(type: string): Completion['type'] {
  if (type === 'function') return 'function';
  if (type === 'object') return 'variable';
  if (type === 'string' || type === 'number' || type === 'boolean') return 'constant';
  return 'variable';
}
