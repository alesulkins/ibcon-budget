import { useNavigate } from 'react-router-dom';
import { Tooltip } from 'antd';
import type { BudgetVersion } from '../types';
import { BUDGET_STATUS_LABELS } from '../types';
import { LINE, BRAND, TEXT_SOFT, RADIUS } from '../theme';

interface Props {
  current: BudgetVersion;
  /** Все версии бюджета проекта — из того же запроса, что и таблица. */
  versions: BudgetVersion[];
}

/**
 * Переключатель между версией-источником и версией-копией.
 *
 * Новая версия создаётся копированием предыдущей, и при сверке правок
 * приходится смотреть то одну, то другую. Пара связана полем
 * `copied_from`, поэтому цепочку строим в обе стороны: у копии ищем
 * родителя, у родителя — копию.
 *
 * Если версия ни с чем не связана, переключать нечего — не рисуем.
 */
export default function VersionSwitch({ current, versions }: Props) {
  const navigate = useNavigate();

  const parent = versions.find(v => v.id === current.copied_from);
  const child = versions.find(v => v.copied_from === current.id);

  const chain = [parent, current, child].filter(Boolean) as BudgetVersion[];
  if (chain.length < 2) return null;

  return (
    <div style={{
      display: 'inline-flex',
      alignItems: 'center',
      gap: 2,
      padding: 3,
      border: `1px solid ${LINE}`,
      borderRadius: RADIUS,
    }}>
      {chain.map((v) => {
        const active = v.id === current.id;
        const label = v.id === current.id && parent ? 'новая'
          : v.id === parent?.id ? 'старая'
          : v.id === child?.id ? 'новая'
          : 'текущая';

        return (
          <Tooltip
            key={v.id}
            title={`Бюджет ${v.project_id}.${v.version_no} — ${
              BUDGET_STATUS_LABELS[v.status] ?? v.status}`}
          >
            <button
              type="button"
              onClick={() => { if (!active) navigate(`/budget-versions/${v.id}`); }}
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
