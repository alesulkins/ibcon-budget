import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

const KEY_PREFIX = 'scroll:';

/**
 * Запоминает позицию прокрутки для каждого адреса и восстанавливает её
 * при возврате — чтобы уход в «Справочники» и обратно не выбрасывал
 * пользователя в начало длинной формы.
 */
export function useScrollRestore() {
  const { pathname } = useLocation();

  useEffect(() => {
    const key = KEY_PREFIX + pathname;

    const save = () => {
      try {
        sessionStorage.setItem(key, String(window.scrollY));
      } catch { /* приватный режим — не критично */ }
    };

    const saved = Number(sessionStorage.getItem(key) ?? '0');
    if (saved > 0) {
      // Контент подгружается асинхронно, и сразу после монтирования
      // страница ещё нулевой высоты — прокручивать некуда. Пробуем
      // несколько кадров подряд, пока высота не позволит.
      let attempts = 0;
      const tryScroll = () => {
        if (window.scrollY === saved) return;
        const reachable = document.body.scrollHeight - window.innerHeight;
        if (reachable >= saved) {
          window.scrollTo(0, saved);
          return;
        }
        if (++attempts < 20) requestAnimationFrame(tryScroll);
      };
      requestAnimationFrame(tryScroll);
    }

    window.addEventListener('scroll', save, { passive: true });
    return () => {
      save();
      window.removeEventListener('scroll', save);
    };
  }, [pathname]);
}
