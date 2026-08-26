import { useRef, useEffect } from 'react';
import { Tooltip } from 'antd';

export interface WizardStepItem {
  key: string;
  title: string;
  desc: string;
}

interface Props {
  items: WizardStepItem[];
  current: number;
  onChange?: (idx: number) => void;
}

const BRAND = '#1a3a6b';
const CIRCLE = 48; // вдвое больше стандартного кружка antd Steps

/**
 * Полоса шагов мастера бюджета.
 *
 * Своя вместо antd Steps по трём причинам: подпись должна стоять ПОД
 * кружком, не переноситься по буквам при нехватке места, а вся полоса —
 * прокручиваться по горизонтали (колесом мыши и ползунком), потому что
 * 15 шагов в ширину экрана не помещаются.
 */
export default function WizardSteps({ items, current, onChange }: Props) {
  const scroller = useRef<HTMLDivElement>(null);
  const activeRef = useRef<HTMLDivElement>(null);

  // Вертикальное колесо прокручивает полосу вбок: на горизонтальной
  // ленте это привычнее, чем искать ползунок.
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (e.deltaY === 0) return;
      const canScroll = el.scrollWidth > el.clientWidth;
      if (!canScroll) return;
      e.preventDefault();
      el.scrollLeft += e.deltaY;
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  }, []);

  // Активный шаг всегда держим в поле зрения.
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest', inline: 'center', behavior: 'smooth' });
  }, [current]);

  return (
    <div
      ref={scroller}
      style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 4,
        overflowX: 'auto',
        overflowY: 'hidden',
        paddingBottom: 8,
        scrollbarWidth: 'thin',
      }}
    >
      {items.map((item, idx) => {
        const done = idx < current;
        const active = idx === current;
        const clickable = !!onChange;

        const bg = active ? BRAND : done ? '#e6f0ff' : '#f5f5f5';
        const fg = active ? '#fff' : done ? BRAND : '#8c8c8c';

        return (
          <div
            key={item.key}
            ref={active ? activeRef : undefined}
            onClick={clickable ? () => onChange(idx) : undefined}
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              // Подпись под кружком, в одну строку, без переноса по буквам
              flex: '0 0 auto',
              width: 130,
              cursor: clickable ? 'pointer' : 'default',
              padding: '4px 2px',
            }}
          >
            <div
              style={{
                width: CIRCLE,
                height: CIRCLE,
                borderRadius: '50%',
                background: bg,
                color: fg,
                border: active ? 'none' : `1px solid ${done ? BRAND : '#d9d9d9'}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 18,
                fontWeight: 600,
                flexShrink: 0,
                transition: 'background 0.2s, color 0.2s',
              }}
            >
              {idx + 1}
            </div>

            <Tooltip title={item.desc}>
              <div
                style={{
                  marginTop: 8,
                  fontSize: 12,
                  lineHeight: 1.3,
                  textAlign: 'center',
                  color: active ? BRAND : '#595959',
                  fontWeight: active ? 600 : 400,
                  // Ключевое: не рвать слова по буквам
                  whiteSpace: 'nowrap',
                  wordBreak: 'normal',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  maxWidth: '100%',
                }}
              >
                {item.title}
              </div>
            </Tooltip>
          </div>
        );
      })}
    </div>
  );
}
