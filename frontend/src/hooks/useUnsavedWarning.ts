import { useEffect } from 'react';
import { useSyncExternalStore } from 'react';
import { subscribeSaveState, getSaveState } from '../store/autosave';

/**
 * Предупреждение о несохранённых данных при закрытии вкладки.
 *
 * Автосохранение обычно успевает записать всё за секунду после ввода и
 * досылает несохранённое при уходе со страницы, но два случая оно не
 * закрывает: закрытие вкладки в течение этой секунды и упавший запрос
 * сохранения. Здесь мы держим браузерное подтверждение ровно для них —
 * когда состояние «сохраняется» или «не сохранено».
 *
 * Текст подтверждения задаёт браузер, свой показать нельзя.
 */
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
