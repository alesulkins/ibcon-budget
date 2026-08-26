import { useEffect, useState } from 'react';

/**
 * Состояние, переживающее уход со страницы и возврат на неё.
 *
 * Нужно, чтобы переход в «Справочники» и обратно не сбрасывал открытый шаг
 * мастера, фильтры реестра и т.п. Живёт в sessionStorage: это состояние
 * одной рабочей сессии, между вкладками и перезапусками браузера его
 * тянуть не надо.
 *
 * Данные форм здесь НЕ хранятся — их сохраняет автосохранение, которое
 * при уходе со страницы досылает несохранённое на сервер (useAutosave).
 */
export function useStickyState<T>(key: string, initial: T) {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = sessionStorage.getItem(key);
      return raw === null ? initial : (JSON.parse(raw) as T);
    } catch {
      return initial;
    }
  });

  useEffect(() => {
    try {
      sessionStorage.setItem(key, JSON.stringify(value));
    } catch {
      // Приватный режим или переполнение — не повод ронять экран.
    }
  }, [key, value]);

  return [value, setValue] as const;
}
