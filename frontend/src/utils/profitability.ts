/**
 * Единая шкала рентабельности — одна на все экраны, где показывается
 * этот показатель: реестр проектов, карточка проекта, шапка версии
 * бюджета и результаты расчёта.
 *
 * Границы заданы владельцем 2026-08-27. Проверяются сверху вниз, нижняя
 * граница включается: 20 % — уже «высокорентабельный», 5 % — «средне-».
 *
 * Цвета — из палитры статусов (theme.ts): владелец назвал четыре
 * значения на шесть ступеней, поэтому две промежуточные выведены из
 * тех же четырёх, а не взяты со стороны:
 *
 *   > 30   #2F7D3A  зелёный STATUS.green — как указано
 *   20-30  #4E9455  тот же зелёный светлее: ступень отличима, но семья та же
 *   5-20   #B07A12  янтарный STATUS.amber — как указано
 *   1-5    #A8621A  оранжевый STATUS.orange — как указано
 *   0-1    #8E4A2A  между оранжевым и красным: порог рентабельности
 *   < 0    #9C2B2B  красный STATUS.red — как указано
 */
export interface ProfitabilityGrade {
  color: string;
  label: string;
}

const SCALE: { min: number; grade: ProfitabilityGrade }[] = [
  { min: 30, grade: { color: '#2F7D3A', label: 'Сверхприбыльный' } },
  { min: 20, grade: { color: '#4E9455', label: 'Высокорентабельный' } },
  { min: 5, grade: { color: '#B07A12', label: 'Среднерентабельный' } },
  { min: 1, grade: { color: '#A8621A', label: 'Низкорентабельный' } },
  { min: 0, grade: { color: '#8E4A2A', label: 'Порог рентабельности' } },
];

const LOSS: ProfitabilityGrade = { color: '#9C2B2B', label: 'Убыточный' };

export function profitabilityGrade(n: number): ProfitabilityGrade {
  // «> 30 %» — строго больше, остальные пороги включают нижнюю границу.
  if (n > 30) return SCALE[0].grade;
  return SCALE.slice(1).find(s => n >= s.min)?.grade ?? LOSS;
}
