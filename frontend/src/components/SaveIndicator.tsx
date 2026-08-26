import { useEffect, useState, useSyncExternalStore } from 'react';
import { subscribeSaveState, getSaveState } from '../store/autosave';

/** Как часто пересчитываем «N сек назад». */
const TICK_MS = 5_000;

function ago(savedAt: number): string {
  const sec = Math.max(0, Math.round((Date.now() - savedAt) / 1000));
  if (sec < 60) return `сохранено ${sec} сек назад`;

  const min = Math.floor(sec / 60);
  if (min < 60) {
    const word = min % 10 === 1 && min % 100 !== 11 ? 'минуту'
      : min % 10 >= 2 && min % 10 <= 4 && (min % 100 < 11 || min % 100 > 14) ? 'минуты'
      : 'минут';
    return `сохранено ${min} ${word} назад`;
  }
  return 'сохранено больше часа назад';
}

/**
 * Индикатор автосохранения: снизу по центру, мелким полупрозрачным шрифтом.
 * Тост при сохранении сознательно не показываем — решение владельца.
 */
export default function SaveIndicator() {
  const state = useSyncExternalStore(subscribeSaveState, getSaveState);

  // Тик, чтобы «N сек назад» обновлялось без новых сохранений.
  const [, setTick] = useState(0);
  useEffect(() => {
    if (state.status !== 'saved') return;
    const id = window.setInterval(() => setTick(t => t + 1), TICK_MS);
    return () => window.clearInterval(id);
  }, [state.status, state.savedAt]);

  if (state.status === 'idle') return null;

  let text = '';
  let color = 'rgba(0, 0, 0, 0.35)';

  if (state.status === 'saving') {
    text = 'сохранение…';
  } else if (state.status === 'error') {
    text = `не сохранено: ${state.error}`;
    color = 'rgba(255, 77, 79, 0.85)';
  } else if (state.savedAt) {
    text = ago(state.savedAt);
  }

  if (!text) return null;

  return (
    <div
      style={{
        position: 'fixed',
        bottom: 12,
        left: 0,
        right: 0,
        display: 'flex',
        justifyContent: 'center',
        pointerEvents: 'none',
        zIndex: 20,
      }}
    >
      <span style={{ fontSize: 12, color, userSelect: 'none' }}>
        {text}
      </span>
    </div>
  );
}
