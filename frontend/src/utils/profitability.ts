// Единая шкала рентабельности — одна на все экраны, где показывается этот
// показатель: реестр проектов, карточка проекта, шапка версии бюджета и
// результаты расчёта.
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
