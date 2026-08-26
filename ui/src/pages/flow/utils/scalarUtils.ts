
export function markExpressionModalClosing() {
  document.body.dataset.expressionModalClosing = 'true';
  window.setTimeout(() => {
    delete document.body.dataset.expressionModalClosing;
  }, 120);
}

export function parseScalarValue(value: string, schemaType?: string): unknown {
  if (isFullPlaceholder(value)) return value;
  if (schemaType === 'boolean') {
    if (value === '') return undefined;
    if (value.toLowerCase() === 'true') return true;
    if (value.toLowerCase() === 'false') return false;
    return value;
  }
  if (schemaType === 'integer' || schemaType === 'number') {
    if (value === '') return undefined;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : value;
  }
  return value;
}

export function containsPlaceholder(value: string) {
  return /\$\{[^{}]+\}/.test(value);
}

export function isFullPlaceholder(value: string) {
  return /^\$\{[^{}]+\}$/.test(value.trim());
}

export function scalarKindForSchemaType(schemaType?: string) {
  if (schemaType === 'boolean') return 'boolean';
  if (schemaType === 'integer' || schemaType === 'number') return 'number';
  return 'string';
}

export function isValidScalarText(value: string, kind: string) {
  const trimmed = value.trim();
  if (kind === 'string') return true;
  if (kind === 'placeholder') return trimmed === '' || isFullPlaceholder(trimmed);
  if (trimmed === '' || isFullPlaceholder(trimmed)) return true;
  if (kind === 'boolean') return trimmed === 'true' || trimmed === 'false';
  if (kind === 'number') return /^-?\d+(\.\d+)?$/.test(trimmed);
  return true;
}
