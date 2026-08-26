export const defaultEndJourneyTypes = new Set(['Success', 'Failure', 'HTTPFinishResponse', 'End']);
export const historyLimit = 80;
export const selectableAPIBasePath = import.meta.env.VITE_JOURNEY_API_BASE_PATH || '/journey';
export const journeyTypes = ['auth', 'resource', 'workflow'] as const;
