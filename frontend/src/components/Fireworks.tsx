import { useEffect, useRef, useState } from 'react';

/** Кому показываем салют — сверяем по полному ФИО. */
const CELEBRATED_FULL_NAME = 'Ярулина Анжела Ильгамовна';

/** Ключ с датой последнего показа, чтобы салют был раз в день. */
const LAST_SHOWN_KEY = 'fireworks:lastShown';

const DURATION_MS = 2_000;
const FADE_MS = 600;

function today(): string {
  return new Date().toISOString().slice(0, 10);
}

/**
 * Показывать ли салют: только нужному пользователю и только один раз
 * за календарный день.
 */
export function shouldShowFireworks(fullName: string | null | undefined): boolean {
  if ((fullName ?? '').trim() !== CELEBRATED_FULL_NAME) return false;
  try {
    return localStorage.getItem(LAST_SHOWN_KEY) !== today();
  } catch {
    // Приватный режим: показать один раз лучше, чем не показать вовсе.
    return true;
  }
}

export function markFireworksShown() {
  try {
    localStorage.setItem(LAST_SHOWN_KEY, today());
  } catch { /* нет доступа к хранилищу — не страшно */ }
}

interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  life: number;
  color: string;
}

const COLORS = ['#ff4d4f', '#ffa940', '#ffec3d', '#73d13d', '#40a9ff', '#9254de', '#f759ab'];

/**
 * Салют на canvas — без внешних библиотек, чтобы не тянуть зависимость
 * ради двух секунд анимации. Слой не перехватывает клики.
 */
export default function Fireworks({ onDone }: { onDone?: () => void }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [fading, setFading] = useState(false);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    const resize = () => {
      canvas.width = window.innerWidth * dpr;
      canvas.height = window.innerHeight * dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener('resize', resize);

    const particles: Particle[] = [];

    const burst = (x: number, y: number) => {
      const color = COLORS[Math.floor(Math.random() * COLORS.length)];
      const count = 44;
      for (let i = 0; i < count; i++) {
        const angle = (Math.PI * 2 * i) / count + Math.random() * 0.2;
        const speed = 2 + Math.random() * 3.4;
        particles.push({
          x, y,
          vx: Math.cos(angle) * speed,
          vy: Math.sin(angle) * speed,
          life: 1,
          color,
        });
      }
    };

    // Первый залп сразу, дальше — пока идёт анимация
    const fire = () => burst(
      window.innerWidth * (0.2 + Math.random() * 0.6),
      window.innerHeight * (0.15 + Math.random() * 0.4),
    );
    fire();
    const burstTimer = window.setInterval(fire, 280);

    let raf = 0;
    const tick = () => {
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);
      for (let i = particles.length - 1; i >= 0; i--) {
        const p = particles[i];
        p.x += p.vx;
        p.y += p.vy;
        p.vy += 0.045;      // притяжение
        p.vx *= 0.99;       // сопротивление воздуха
        p.life -= 0.012;
        if (p.life <= 0) {
          particles.splice(i, 1);
          continue;
        }
        ctx.globalAlpha = Math.max(0, p.life);
        ctx.fillStyle = p.color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 2.6, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);

    const stopTimer = window.setTimeout(() => {
      window.clearInterval(burstTimer);
      setFading(true);
      window.setTimeout(() => onDone?.(), FADE_MS);
    }, DURATION_MS);

    return () => {
      window.removeEventListener('resize', resize);
      window.clearInterval(burstTimer);
      window.clearTimeout(stopTimer);
      cancelAnimationFrame(raf);
    };
  }, [onDone]);

  return (
    <canvas
      ref={canvasRef}
      style={{
        position: 'fixed',
        inset: 0,
        // Слой поверх страницы, но полностью прозрачный для мыши
        pointerEvents: 'none',
        zIndex: 2000,
        opacity: fading ? 0 : 1,
        transition: `opacity ${FADE_MS}ms ease-out`,
      }}
    />
  );
}
