import { useNavigate } from 'react-router-dom';
import { Tooltip } from 'antd';
import type { BudgetVersion } from '../types';
import { BUDGET_STATUS_LABELS } from '../types';
import { LINE, BRAND, TEXT_SOFT, RADIUS } from '../theme';
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

  const child = versions.find(v => v.copied_from === current.id);
  const parent = versions.find(v => v.id === current.copied_from);

  // [старая, новая] — всегда две версии, не больше.
  const pair: BudgetVersion[] | null = child
    ? [current, child]
    : parent
      ? [parent, current]
      : null;

  if (!pair) return null;
  const [older, newer] = pair;

  return (
    <div style={{
      display: 'inline-flex',
      alignItems: 'center',
      gap: 2,
      padding: 3,
      border: `1px solid ${LINE}`,
      borderRadius: RADIUS,
    }}>
      {[older, newer].map((v) => {
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
                background: active ? BRAND : 'transparent',
                color: active ? '#FDF9F8' : TEXT_SOFT,
                whiteSpace: 'nowrap',
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
