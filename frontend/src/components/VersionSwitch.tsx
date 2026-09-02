import { useLayoutEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Tooltip } from 'antd';
import type { BudgetVersion } from '../types';
import { BUDGET_STATUS_LABELS } from '../types';
import { LINE, TEXT_SOFT, RADIUS } from '../theme';
import { carryScrollTo } from '../hooks/useScrollRestore';

interface Props {
  current: BudgetVersion;
  /** Все версии бюджета проекта — из того же запроса, что и таблица. */
  versions: BudgetVersion[];
}

/**
 * Переключатель РОВНО МЕЖДУ ДВУМЯ версиями: новой и той, из которой её
 * скопировали. Историю версий здесь не листают.
 *
 * Пара связана полем `copied_from` и одна и та же с обеих сторон:
 * стоя на копии, видим её источник; стоя на источнике — его копию.
 *
 * Когда версия участвует сразу в двух связях (её саму скопировали из
 * прежней, и с неё уже сняли новую), берём СВЕЖУЮ связь — ту, где эта
 * версия старая. Иначе переключатель превратился бы в ленту истории,
 * а сверяют всегда последнюю пару.
 */
export default function VersionSwitch({ current, versions }: Props) {
  const navigate = useNavigate();

  /**
   * Подсветка выбранной версии — отдельная плашка, которая переезжает
   * между кнопками, а не заливка каждой кнопки по очереди.
   *
   * Заливка переключалась мгновенно, и переход читался как рывок.
   * Переезжающая плашка показывает само движение: глаз ведёт её от
   * старой версии к новой и не теряет, что с чем сравнивает. Размеры
   * меряем — подписи версий разной длины («старая · Архив» и
   * «новая · Согласован»), и половиной ширины тут не обойтись.
   */
  const btns = useRef<(HTMLButtonElement | null)[]>([]);
  const [thumb, setThumb] = useState<{ left: number; width: number } | null>(null);

  const child = versions.find(v => v.copied_from === current.id);
  const parent = versions.find(v => v.id === current.copied_from);

  // [старая, новая] — всегда две версии, не больше.
  const pair: BudgetVersion[] | null = child
    ? [current, child]
    : parent
      ? [parent, current]
      : null;

  const activeIdx = pair && pair[1].id === current.id ? 1 : 0;

  useLayoutEffect(() => {
    const el = btns.current[activeIdx];
    if (!el) return;
    const measure = () => setThumb({ left: el.offsetLeft, width: el.offsetWidth });
    measure();
    // Шрифт может доехать позже разметки — тогда кнопка меняет ширину, и
    // плашка должна поехать за ней, а не остаться шире или уже.
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [activeIdx, current.id, versions]);

  if (!pair) return null;
  const [older, newer] = pair;

  return (
    <div
      className="ibcon-version-switch"
      style={{
        position: 'relative',
        display: 'inline-flex',
        alignItems: 'center',
        gap: 2,
        padding: 3,
        border: `1px solid ${LINE}`,
        borderRadius: RADIUS,
      }}
    >
      {thumb && (
        <span
          aria-hidden
          className="ibcon-version-thumb"
          style={{ transform: `translateX(${thumb.left}px)`, width: thumb.width }}
        />
      )}
      {[older, newer].map((v, i) => {
        const active = v.id === current.id;
        const label = v.id === older.id ? 'старая' : 'новая';

        return (
          <Tooltip
            key={v.id}
            title={`Бюджет ${v.project_id}.${v.version_no} — ${
              BUDGET_STATUS_LABELS[v.status] ?? v.status}`}
          >
            <button
              type="button"
              ref={(el) => { btns.current[i] = el; }}
              onClick={() => {
                if (active) return;
                // Позицию прокрутки переносим на соседнюю версию: версии
                // сравнивают, стоя на одном и том же месте длинной формы,
                // и прыжок в начало сбивал бы сравнение.
                const to = `/budget-versions/${v.id}`;
                carryScrollTo(to);
                navigate(to);
              }}
              style={{
                appearance: 'none',
                border: 0,
                borderRadius: RADIUS - 2,
                padding: '4px 12px',
                font: 'inherit',
                fontSize: 12,
                fontWeight: active ? 600 : 400,
                lineHeight: 1.3,
                cursor: active ? 'default' : 'pointer',
                // Цвет переменной, а не константой: фирменный цвет
                // выбирается в настройках, и константа его не знает.
                // Заливки у кнопки нет — её роль играет переезжающая
                // плашка под ней; кнопке остаётся только цвет текста.
                background: 'transparent',
                color: active ? '#FDF9F8' : TEXT_SOFT,
                whiteSpace: 'nowrap',
                position: 'relative',
                zIndex: 1,
                transition: 'color 0.35s ease',
              }}
            >
              {label}
              <span style={{ opacity: 0.75 }}>
                {' · '}{BUDGET_STATUS_LABELS[v.status] ?? v.status}
              </span>
            </button>
          </Tooltip>
        );
      })}
    </div>
  );
}
