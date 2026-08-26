import { useRef, useEffect } from 'react';
import { Tooltip } from 'antd';
import { BRAND } from '../theme';

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
  // Момент, до которого прокрутку считаем ручной: пока пользователь крутит
  // колесом, автоцентрирование молчит.
  const manualUntil = useRef(0);

  // Вертикальное колесо прокручивает полосу вбок: на горизонтальной
  // ленте это привычнее, чем искать ползунок.
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;

    let pending = 0;
    let frame = 0;

    const flush = () => {
      frame = 0;
      el.scrollLeft += pending;
      pending = 0;
    };

    const onWheel = (e: WheelEvent) => {
      // Горизонтальный жест трекпада браузер отрабатывает сам. Если
      // вмешаться, полоса едет дважды — и от браузера, и от нас.
      if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return;
      if (el.scrollWidth <= el.clientWidth) return;
      e.preventDefault();
      manualUntil.current = Date.now() + 400;
      pending += e.deltaY;
      // Все события одного кадра складываются в одну запись scrollLeft.
      // Инерция трекпада шлёт их десятками подряд, и запись на каждое
      // заставляла браузер пересчитывать раскладку по кругу — отсюда и
      // рывки при быстром пролистывании.
      if (!frame) frame = requestAnimationFrame(flush);
    };

    el.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      el.removeEventListener('wheel', onWheel);
      if (frame) cancelAnimationFrame(frame);
    };
  }, []);

  // Активный шаг держим в поле зрения — но только если он из него ушёл.
  useEffect(() => {
    const el = scroller.current;
    const active = activeRef.current;
    if (!el || !active) return;

    const left = active.offsetLeft;
    const right = left + active.offsetWidth;
    if (left >= el.scrollLeft && right <= el.scrollLeft + el.clientWidth) return;

    // scrollTo по самому контейнеру, а не scrollIntoView: последний
    // подкручивает и страницу целиком, из-за чего экран дёргался по
    // вертикали при переключении шага.
    el.scrollTo({
      left: left - (el.clientWidth - active.offsetWidth) / 2,
      // Плавную анимацию не запускаем поверх ручной прокрутки: две
      // анимации на одном контейнере и давали заедание.
      behavior: Date.now() < manualUntil.current ? 'auto' : 'smooth',
    });
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
