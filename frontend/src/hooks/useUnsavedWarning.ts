import { useEffect } from 'react';
import { useSyncExternalStore } from 'react';
import { subscribeSaveState, getSaveState } from '../store/autosave';

// Предупреждение о несохранённых данных при закрытии вкладки.
export function useUnsavedWarning() {
  const state = useSyncExternalStore(subscribeSaveState, getSaveState);
  const dirty = state.status === 'saving' || state.status === 'error';

  useEffect(() => {
    if (!dirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      // Требуется старыми браузерами для показа диалога
      e.returnValue = '';
      return '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [dirty]);
}
