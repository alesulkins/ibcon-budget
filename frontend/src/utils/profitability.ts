/**
 * Единая шкала рентабельности — одна на все экраны, где показывается
 * этот показатель: реестр проектов, карточка проекта, шапка версии
 * бюджета и результаты расчёта.
 *
 * Границы заданы владельцем 2026-08-27. Проверяются сверху вниз, нижняя
 * граница включается: 20 % — уже «высокорентабельный», 5 % — «средне-».
 *
 * Цвета подобраны так, чтобы читаться на белом: чистый жёлтый
 * (#fadb14) на белом фоне неразличим, поэтому середина шкалы —
 * приглушённый жёлтый #d4b106.
 */
export interface ProfitabilityGrade {
  color: string;
  label: string;
}

const SCALE: { min: number; grade: ProfitabilityGrade }[] = [
  { min: 30, grade: { color: '#52c41a', label: 'Сверхприбыльный' } },
  { min: 20, grade: { color: '#73d13d', label: 'Высокорентабельный' } },
  { min: 5, grade: { color: '#d4b106', label: 'Среднерентабельный' } },
  { min: 1, grade: { color: '#fa8c16', label: 'Низкорентабельный' } },
  { min: 0, grade: { color: '#fa541c', label: 'Порог рентабельности' } },
];

const LOSS: ProfitabilityGrade = { color: '#cf1322', label: 'Убыточный' };

export function profitabilityGrade(n: number): ProfitabilityGrade {
  // «> 30 %» — строго больше, остальные пороги включают нижнюю границу.
  if (n > 30) return SCALE[0].grade;
  return SCALE.slice(1).find(s => n >= s.min)?.grade ?? LOSS;
}
