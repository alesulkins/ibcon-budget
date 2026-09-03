import { useEffect, useState } from 'react';

// Состояние, переживающее уход со страницы и возврат на неё. Нужно, чтобы
// переход в «Справочники» и обратно не сбрасывал открытый шаг мастера,
// фильтры реестра и т.п.
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
