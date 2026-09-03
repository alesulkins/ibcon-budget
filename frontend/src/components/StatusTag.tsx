import type { ReactNode } from 'react';
import { STATUS } from '../theme';
import type { StatusColor } from '../theme';

interface Props {
  /** Ключ палитры статусов; неизвестный трактуем как нейтральный. */
  color?: string;
  /** Длинные подписи (журнал изменений) переносим, а не обрезаем. */
  wrap?: boolean;
  children: ReactNode;
}

// Плашка статуса: приглушённый цвет текста на нём же с прозрачностью 15 %.
// Одна на все экраны — реестр, карточка проекта, справочники, пользователи,
// журнал изменений.
export default function StatusTag({ color, wrap, children }: Props) {
  const hex = STATUS[color as StatusColor] ?? STATUS.grey;

  return (
    <span style={{
      display: 'inline-flex',
      alignItems: 'center',
      minHeight: 20,
      padding: wrap ? '3px 10px' : '0 10px',
      borderRadius: 999,
      // 26 в шестнадцатеричной записи — 15 % непрозрачности.
      background: `${hex}26`,
      color: hex,
      fontSize: 11,
      fontWeight: 500,
      lineHeight: wrap ? 1.4 : 1,
      whiteSpace: wrap ? 'normal' : 'nowrap',
    }}>
      {children}
    </span>
  );
}
