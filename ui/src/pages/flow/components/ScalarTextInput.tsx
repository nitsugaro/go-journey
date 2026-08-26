import type { ReactNode } from 'react';
import { HighlightedTextEditor, type HighlightLanguage } from './CodeEditors';

export function ScalarTextInput({
  value,
  onChange,
  multiline,
  language,
  invalid,
  placeholder,
  action,
}: {
  value: string;
  onChange: (value: string) => void;
  multiline?: boolean;
  language?: HighlightLanguage;
  invalid?: boolean;
  placeholder?: string;
  action?: ReactNode;
}) {
  return (
    <HighlightedTextEditor
      value={value}
      onChange={onChange}
      multiline={multiline}
      language={language || 'text'}
      invalid={invalid}
      placeholder={placeholder}
      action={action}
      className="mt-1"
    />
  );
}
