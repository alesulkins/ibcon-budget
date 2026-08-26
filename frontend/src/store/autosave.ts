/**
 * Общее состояние автосохранения мастера бюджета.
 *
 * Формы шагов сообщают сюда о ходе сохранения, а индикатор внизу экрана
 * его показывает. Через модуль, а не через контекст, чтобы форма шага и
 * индикатор не были обязаны жить в одном поддереве.
 */

export type SaveStatus = 'idle' | 'saving' | 'saved' | 'error';

export interface SaveState {
  status: SaveStatus;
  /** Момент последнего успешного сохранения, мс. */
  savedAt: number | null;
  error: string;
}

let state: SaveState = { status: 'idle', savedAt: null, error: '' };

const listeners = new Set<() => void>();

function emit(next: SaveState) {
  state = next;
  listeners.forEach(l => l());
}

export function subscribeSaveState(listener: () => void): () => void {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

export function getSaveState(): SaveState {
  return state;
}

export function markSaving() {
  emit({ ...state, status: 'saving', error: '' });
}

export function markSaved() {
  emit({ status: 'saved', savedAt: Date.now(), error: '' });
}

export function markSaveError(error: string) {
  emit({ ...state, status: 'error', error });
}

/** Сброс при уходе с версии бюджета, чтобы индикатор не тянул чужое время. */
export function resetSaveState() {
  emit({ status: 'idle', savedAt: null, error: '' });
}
