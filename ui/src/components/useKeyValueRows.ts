import { useCallback, useEffect, useRef, useState } from 'react';

export type KeyValueRow<T> = {
  id: string;
  key: string;
  value: T;
};

export function useKeyValueRows<T>(
  value: Record<string, T>,
  onChange: (value: Record<string, T>) => void,
) {
  const idSequence = useRef(0);
  const nextID = useCallback(() => `key-value-row-${++idSequence.current}`, []);
  const rowsFromValue = useCallback(
    (source: Record<string, T>) =>
      Object.entries(source).map(([key, entryValue]) => ({ id: nextID(), key, value: entryValue })),
    [nextID],
  );
  const [rows, setRows] = useState<KeyValueRow<T>[]>(() => rowsFromValue(value));
  const valueSignature = JSON.stringify(Object.entries(value));
  const valueRef = useRef(value);
  valueRef.current = value;
  const lastPublishedSignature = useRef(valueSignature);

  useEffect(() => {
    if (lastPublishedSignature.current === valueSignature) {
      lastPublishedSignature.current = '';
      return;
    }
    setRows(rowsFromValue(valueRef.current));
  }, [rowsFromValue, valueSignature]);

  function publish(nextRows: KeyValueRow<T>[]) {
    const nextValue = Object.fromEntries(
      nextRows
        .map((row) => [row.key.trim(), row.value] as const)
        .filter(([key]) => Boolean(key)),
    ) as Record<string, T>;
    lastPublishedSignature.current = JSON.stringify(Object.entries(nextValue));
    onChange(nextValue);
  }

  function add(key: string, entryValue: T) {
    const nextRows = [...rows, { id: nextID(), key, value: entryValue }];
    setRows(nextRows);
    publish(nextRows);
  }

  function update(id: string, updater: (row: KeyValueRow<T>) => KeyValueRow<T>) {
    const nextRows = rows.map((row) => (row.id === id ? updater(row) : row));
    setRows(nextRows);
    publish(nextRows);
  }

  function remove(id: string) {
    const nextRows = rows.filter((row) => row.id !== id);
    setRows(nextRows);
    publish(nextRows);
  }

  return { rows, add, update, remove };
}
