export function formatStepTypeLabel(stepType: string) {
  return stepType
    .replace(/[-\s]+/g, '_')
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1_$2')
    .replace(/_{2,}/g, '_')
    .toUpperCase()
}
