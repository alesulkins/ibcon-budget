import { BUDGET_STATUS_LABELS, PROJECT_STATUS_LABELS } from '../types';
import type { AuditEntry } from '../types';

/**
 * Человеческая расшифровка колонки «Что изменено».
 *
 * В базе комментарий лежит сырым: ключ ввода (`rent_apartments`), код
 * статуса с двоеточием (`under_review: причина`), голый id проекта или
 * вовсе пусто. Здесь всё это переводится на язык интерфейса.
 *
 * Расшифровка нужна и для будущих записей, и для уже накопленных:
 * триста строк журнала переписывать в базе нельзя — это журнал аудита.
 */

/**
 * Ключ ввода → лист формы, как он называется в мастере бюджета.
 * Ключи те же, что принимает PUT /budget-versions/:vid/inputs/:type.
 */
const INPUT_TITLES: Record<string, string> = {
  employees: 'Сотрудники: ФОТ и билеты (4.6)',
  bonuses: 'Премии и компенсации (4.1)',
  rent_apartments: 'Аренда квартир и риелтор (4.2)',
  transport: 'Транспорт: авто и гараж (4.3)',
  wagonciks: 'Стройплощадка: вагончики (4.4)',
  office: 'Офис: аренда и уборка (4.5)',
  equipment_items: 'Приборы стройконтроля (4.7)',
  software_items: 'Приобретение ПО (4.8)',
  subcontract_ext_items: 'Субподряд, ГПХ внешний (4.9)',
  gph_employees: 'Субподряд, ГПХ сотрудников (4.10)',
  subcontract_gen_items: 'Субподрядные работы (4.11)',
  corporate_events_items: 'Корпоративные мероприятия (4.12)',
  budget_params: 'Параметры бюджета: ТКП, непредвиденные, АУП, БГ',

  // Прочие накладные — каждая статья отдельным ключом.
  overtime_rf: 'Переработки сотрудников РФ',
  overtime_kg: 'Переработки сотрудников КГ',
  internet: 'Интернет',
  mobile: 'Мобильная связь',
  lab_research: 'Лабораторные исследования',
  training: 'Обучение персонала',
  medical: 'Медицинский осмотр',
  uniform: 'Спецодежда',
  computers: 'Приобретение ПК и оргтехники',
  furniture: 'Приобретение мебели',
  office_supplies: 'Содержание офиса (канцтовары)',
  postal: 'Почтовые расходы',
  fuel: 'ГСМ',
  transport_services: 'Транспортные услуги',
  subcontract_org: 'Субподряд (организационные улучшения)',
  representative: 'Представительские расходы',
  bank_services: 'Услуги банков',
  insurance_liab: 'Страхование ответственности',
  utilities: 'Коммунальные расходы',
  security: 'Охрана объекта',
  auto_insurance: 'Страхование КАСКО и ОСАГО',

  // Ключи прежнего формата: данные в них вводились готовыми суммами.
  // Записи о них остались в журнале, поэтому подписи нужны.
  realtor: 'Риелтор (старый формат)',
  office_rent: 'Аренда офиса (старый формат)',
  office_cleaning: 'Уборка офиса (старый формат)',
  transport_rental: 'Аренда транспорта (старый формат)',
  garage_rent: 'Аренда гаража (старый формат)',
  site_setup: 'Обустройство площадки (старый формат)',
  software: 'Приобретение ПО (старый формат)',
  subcontract_ext: 'ГПХ внешний (старый формат)',
  subcontract_gen: 'Субподрядные работы (старый формат)',
  subcontract_emp: 'ГПХ сотрудников (старый формат)',
  control_equipment: 'Приборы стройконтроля (старый формат)',
  corporate_events: 'Корпоративы (старый формат)',
};

/** Разбирает «статус: причина» и переводит код статуса в подпись. */
function statusChange(comment: string, labels: Record<string, string>): string {
  const at = comment.indexOf(':');
  if (at < 0) return labels[comment.trim()] ?? comment;

  const code = comment.slice(0, at).trim();
  const reason = comment.slice(at + 1).trim();
  const title = labels[code] ?? code;
  return reason ? `${title} — ${reason}` : title;
}

/**
 * Что показать в колонке «Что изменено». Пустая строка — показывать
 * нечего: подпись действия уже всё сказала.
 */
export function auditChangeText(e: AuditEntry): string {
  const c = (e.comment ?? '').trim();

  switch (e.action) {
    case 'save_budget_input':
      return INPUT_TITLES[c] ?? (c ? `Данные листа «${c}»` : '');

    case 'change_budget_status':
      return statusChange(c, BUDGET_STATUS_LABELS);

    case 'change_project_status':
      return statusChange(c, PROJECT_STATUS_LABELS);

    case 'grant_project_access':
    case 'revoke_project_access':
      // В старых записях лежит голый id ПОЛЬЗОВАТЕЛЯ, которому выдали
      // или у которого отозвали доступ (проект — в колонке «Сущность»).
      // Новые записи пишут имя и почту.
      if (/^\d+$/.test(c)) return `Пользователь №${c}`;
      return c;

    default:
      return c;
  }
}
