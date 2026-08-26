import { useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import CodeMirror from '@uiw/react-codemirror';
import { json } from '@codemirror/lang-json';
import { javascript } from '@codemirror/lang-javascript';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { Decoration, EditorView, keymap, MatchDecorator, ViewPlugin, type ViewUpdate } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { tags } from '@lezer/highlight';
import { markExpressionModalClosing } from '../utils/scalarUtils';

export type HighlightLanguage = 'text' | 'expression' | 'json';

const codeEditorHighlight = HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--color-code-keyword)', fontWeight: '600' },
  { tag: [tags.atom, tags.bool, tags.null], color: 'var(--color-code-atom)', fontWeight: '600' },
  { tag: tags.number, color: 'var(--color-code-number)' },
  { tag: tags.string, color: 'var(--color-code-string)' },
  { tag: tags.variableName, color: 'var(--color-code-variable)' },
  { tag: tags.propertyName, color: 'var(--color-code-property)', fontWeight: '600' },
  { tag: tags.operator, color: 'var(--color-code-operator)', fontWeight: '700' },
  { tag: tags.function(tags.variableName), color: 'var(--color-blue)', fontWeight: '700' },
  { tag: tags.punctuation, color: 'var(--color-muted-faint)' },
  { tag: tags.bracket, color: 'var(--color-muted-soft)' },
]);

const codeEditorTheme = EditorView.theme({
  '&': {
    backgroundColor: 'var(--color-surface-subtle)',
    color: 'var(--color-ink)',
    borderRadius: '1rem',
    outline: 'none !important',
    height: '100%',
  },
  '&.cm-focused': {
    outline: 'none !important',
    backgroundColor: 'var(--color-surface)',
  },
  '.cm-editor, .cm-editor.cm-focused': {
    backgroundColor: 'transparent',
    color: 'var(--color-ink)',
    outline: 'none !important',
  },
  '.cm-gutters': {
    backgroundColor: 'transparent',
    color: 'var(--color-muted-soft)',
    border: 'none',
  },
  '.cm-scroller': {
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
    fontSize: '0.875rem',
    lineHeight: '1.5rem',
    backgroundColor: 'transparent',
    color: 'var(--color-ink)',
  },
  '.cm-content': {
    padding: '0.625rem 0.75rem',
    minHeight: '100%',
    caretColor: 'var(--color-ink)',
    color: 'var(--color-ink)',
    backgroundColor: 'transparent',
  },
  '.cm-line': {
    padding: 0,
    color: 'var(--color-ink)',
  },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
    backgroundColor: 'var(--color-blue-selection)',
  },
  '.cm-cursor': {
    borderLeftColor: 'var(--color-ink)',
    borderLeftWidth: '2px',
  },
  '.cm-placeholder': {
    color: 'var(--color-muted-soft)',
  },
  '.cm-placeholder-token': {
    backgroundColor: 'color-mix(in srgb, var(--color-warning-soft) 72%, transparent)',
    color: 'var(--color-warning)',
    borderRadius: '0.35rem',
    fontWeight: '700',
    padding: '0 0.1rem',
    boxShadow: 'inset 0 -1px 0 var(--color-warning-border)',
  },
});

const placeholderMatcher = new MatchDecorator({
  regexp: /\$\{[^{}]+\}/g,
  decoration: Decoration.mark({ class: 'cm-placeholder-token' }),
});

const placeholderHighlight = ViewPlugin.fromClass(
  class {
    decorations;

    constructor(view: EditorView) {
      this.decorations = placeholderMatcher.createDeco(view);
    }

    update(update: ViewUpdate) {
      this.decorations = placeholderMatcher.updateDeco(update, this.decorations);
    }
  },
  {
    decorations: (plugin) => plugin.decorations,
  },
);

const stopEditorEventPropagation = EditorView.domEventHandlers({
  pointerdown: (event) => {
    event.stopPropagation();
    return false;
  },
  click: (event) => {
    event.stopPropagation();
    return false;
  },
  mousedown: (event) => {
    event.stopPropagation();
    return false;
  },
  mouseup: (event) => {
    event.stopPropagation();
    return false;
  },
  dblclick: (event) => {
    event.stopPropagation();
    return false;
  },
});

const singleLineCodeEditor = [
  keymap.of([{ key: 'Enter', run: () => true }]),
  EditorState.changeFilter.of((transaction) => {
    if (!transaction.docChanged) return true;
    return !transaction.newDoc.toString().includes('\n');
  }),
  EditorView.domEventHandlers({
    beforeinput: (event) => {
      const inputEvent = event as InputEvent;
      if (inputEvent.inputType === 'insertLineBreak') {
        event.preventDefault();
        return true;
      }
      return false;
    },
    paste: (event, view) => {
      const pasted = event.clipboardData?.getData('text');
      if (!pasted || !pasted.includes('\n')) return false;
      event.preventDefault();
      view.dispatch(view.state.replaceSelection(pasted.replace(/\r?\n/g, ' ')));
      return true;
    },
  }),
];

export function HighlightedTextEditor({
  value,
  onChange,
  multiline,
  language,
  invalid,
  placeholder,
  className,
  autoFocus,
  action,
}: {
  value: string;
  onChange: (value: string) => void;
  multiline?: boolean;
  language?: HighlightLanguage;
  invalid?: boolean;
  placeholder?: string;
  className?: string;
  autoFocus?: boolean;
  action?: ReactNode;
}) {
  const wrapperClass = ['relative', className].filter(Boolean).join(' ');
  const highlighted = Boolean(language);
  const inputClass = [
    'w-full resize-none rounded-2xl border border-transparent bg-[var(--color-surface-subtle)] py-2.5 pl-3 text-sm normal-case tracking-normal text-[var(--color-ink)] outline-none transition selection:bg-[var(--color-blue-selection)]',
    action ? 'pr-12' : 'pr-3',
    invalid
      ? 'ring-2 ring-[var(--color-red)]'
      : 'ring-1 ring-[var(--color-border-soft)] focus:bg-[var(--color-surface)] focus:ring-2 focus:ring-[var(--color-blue)]',
    multiline ? 'h-full min-h-24 leading-6' : 'h-[42px]',
  ].join(' ');

  if (highlighted) {
    return (
      <div
        className={[
          wrapperClass,
          action && !multiline
            ? 'flex h-[42px] items-center overflow-hidden rounded-2xl bg-[var(--color-surface-subtle)] ring-1 ring-[var(--color-border-soft)] focus-within:bg-[var(--color-surface)] focus-within:ring-2 focus-within:ring-[var(--color-blue)]'
            : '',
          action && !multiline && invalid ? 'ring-2 ring-[var(--color-red)]' : '',
        ].filter(Boolean).join(' ')}
      >
        <div className={action && !multiline ? 'min-w-0 flex-1' : undefined}>
          <CodeMirror
            value={multiline ? value : value.replace(/\r?\n/g, ' ')}
            onChange={(next) => onChange(multiline ? next : next.replace(/\r?\n/g, ' '))}
            placeholder={placeholder}
            autoFocus={autoFocus}
            basicSetup={{
              lineNumbers: false,
              foldGutter: false,
              highlightActiveLine: false,
              highlightActiveLineGutter: false,
              autocompletion: false,
              searchKeymap: false,
            }}
            extensions={[
              ...(language === 'json'
                ? [json(), syntaxHighlighting(codeEditorHighlight), placeholderHighlight]
                : language === 'expression'
                  ? [javascript({ jsx: false, typescript: false }), syntaxHighlighting(codeEditorHighlight), placeholderHighlight]
                  : [placeholderHighlight]),
              codeEditorTheme,
              stopEditorEventPropagation,
              ...(multiline ? [EditorView.lineWrapping] : singleLineCodeEditor),
            ]}
            className={[
              'overflow-hidden normal-case tracking-normal transition',
              action && !multiline ? 'journey-inline-code-input h-[42px] rounded-none bg-transparent' : 'rounded-2xl',
              invalid && !(action && !multiline) ? 'ring-2 ring-[var(--color-red)]' : '',
              !invalid && !(action && !multiline) ? 'ring-1 ring-[var(--color-border-soft)]' : '',
              multiline ? 'h-full min-h-24' : 'h-[42px]',
            ].join(' ')}
          />
        </div>
        {action && (
          <span className={multiline ? 'absolute right-2 top-1/2 z-10 -translate-y-1/2' : 'flex h-full shrink-0 items-center px-2'}>
            {action}
          </span>
        )}
      </div>
    );
  }

  return multiline ? (
    <div className={wrapperClass}>
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={inputClass}
        spellCheck={false}
        placeholder={placeholder}
        autoFocus={autoFocus}
      />
      {action && <span className="absolute right-2 top-1/2 z-10 -translate-y-1/2">{action}</span>}
    </div>
  ) : (
    <div className={wrapperClass}>
      <input
        value={value.replace(/\r?\n/g, ' ')}
        onChange={(event) => onChange(event.target.value.replace(/\r?\n/g, ' '))}
        className={inputClass}
        spellCheck={false}
        placeholder={placeholder}
        autoFocus={autoFocus}
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.preventDefault();
        }}
      />
      {action && <span className="absolute right-2 top-1/2 z-10 -translate-y-1/2">{action}</span>}
    </div>
  );
}

export function ExpandedEditorModal({
  title,
  value,
  language,
  onChange,
  onClose,
}: {
  title: string;
  value: string;
  language?: HighlightLanguage;
  onChange: (value: string) => void;
  onClose: () => void;
}) {
  const editorLanguage = language || 'text';
  const modalMultiline = editorLanguage !== 'expression';
  const [closing, setClosing] = useState(false);
  const requestClose = () => {
    if (closing) return;
    setClosing(true);
    window.setTimeout(onClose, 140);
  };
  return createPortal(
    <div
      className="motion-modal-backdrop fixed inset-0 z-[1000] flex items-center justify-center bg-[var(--color-overlay)] p-6"
      data-closing={closing ? 'true' : undefined}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) markExpressionModalClosing();
      }}
      onClick={requestClose}
    >
      <div
        className="motion-modal-surface flex h-[min(780px,92vh)] w-[min(1120px,94vw)] flex-col overflow-hidden rounded-[2rem] bg-[var(--color-surface)] shadow-2xl"
        onClick={(event) => event.stopPropagation()}
        data-expression-modal
      >
        <div className="flex items-center justify-between border-b border-[var(--color-border-soft)] px-6 py-5">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-blue)]">
              {editorLanguage === 'expression' ? 'Expression editor' : 'Value editor'}
            </p>
            <h3 className="text-lg font-semibold text-[var(--color-ink)]">{title}</h3>
          </div>
        </div>
        <div className="min-h-0 flex-1 bg-[var(--color-surface-muted-transparent)] p-6">
          <div className="flex h-full min-h-[540px] flex-col overflow-hidden rounded-3xl border border-[var(--color-border-subtle)] bg-[var(--color-surface)] shadow-inner">
            <div className="flex items-center justify-between border-b border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] px-4 py-2">
              <div className="flex items-center gap-1.5">
                <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-red-border)]" />
                <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-warning-border)]" />
                <span className="h-2.5 w-2.5 rounded-full bg-[var(--color-green-border)]" />
              </div>
              <span className="rounded-full bg-[var(--color-surface)] px-2.5 py-1 font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-muted-soft)] ring-1 ring-[var(--color-border-soft)]">
                {editorLanguage}
              </span>
            </div>
            <HighlightedTextEditor
              value={value}
              onChange={onChange}
              language={editorLanguage}
              multiline={modalMultiline}
              className={modalMultiline ? 'min-h-0 flex-1 rounded-none bg-[var(--color-surface)]' : 'px-4 py-5'}
              autoFocus
            />
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}
