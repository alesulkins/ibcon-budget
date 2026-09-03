import { useEffect } from 'react';
import {
  INACTIVITY_LIMIT_MS, touchActivity, idleMs, isLoggedIn, logout,
} from '../store/auth';

/** Как часто сверяем простой с лимитом. */
const CHECK_INTERVAL_MS = 15_000;

/** Не чаще одной отметки активности в 10 с — иначе mousemove забьёт localStorage. */
const TOUCH_THROTTLE_MS = 10_000;

const ACTIVITY_EVENTS = [
  'mousedown', 'keydown', 'wheel', 'touchstart', 'scroll',
] as const;

// Автовыход при бездействии — 60 минут (ТЗ 3.8 п.6). Клиентская половина
// проверки; серверная живёт в middleware.Auth и опирается на
// users.last_activity.
export function useInactivityLogout() {
  useEffect(() => {
    if (!isLoggedIn()) return;

    let lastTouch = 0;
    const onActivity = () => {
      const now = Date.now();
      if (now - lastTouch < TOUCH_THROTTLE_MS) return;
      lastTouch = now;
      touchActivity();
    };

    ACTIVITY_EVENTS.forEach(e =>
      window.addEventListener(e, onActivity, { passive: true }));

    const timer = window.setInterval(() => {
      if (!isLoggedIn()) return;
      if (idleMs() > INACTIVITY_LIMIT_MS) {
        logout('timeout');
      }
    }, CHECK_INTERVAL_MS);

    // Возврат на вкладку после долгого простоя — проверяем сразу,
    // не дожидаясь очередного тика интервала.
    const onVisible = () => {
      if (document.visibilityState === 'visible'
        && isLoggedIn() && idleMs() > INACTIVITY_LIMIT_MS) {
        logout('timeout');
      }
    };
    document.addEventListener('visibilitychange', onVisible);

    return () => {
      ACTIVITY_EVENTS.forEach(e => window.removeEventListener(e, onActivity));
      document.removeEventListener('visibilitychange', onVisible);
      window.clearInterval(timer);
    };
  }, []);
}
