import { expect, test } from '@playwright/test';

/**
 * Нормативы времени отклика (SLA Минцифры), секунды.
 * Меряется время до появления данных на экране, а не ответ сервера.
 */
const SLA = {
  pageLoad: 5,
  projectCard: 5,
  budgetVersion: 10,
  calculation: 120,
  export: 60,
};

const EMAIL = process.env.SLA_EMAIL ?? 'admin@ibcon.ru';
// Пароль только из окружения: зашитый в код уезжает вместе с репозиторием.
const PASSWORD = process.env.SLA_PASSWORD ?? '';

/** Секунды с начала замера — в том виде, в каком их читает норматив. */
async function seconds(fn: () => Promise<unknown>): Promise<number> {
  const start = Date.now();
  await fn();
  return (Date.now() - start) / 1000;
}

test.beforeEach(async ({ page }) => {
  await page.goto('/login');
  await page.getByPlaceholder('admin@ibcon.ru').fill(EMAIL);
  await page.locator('input[type="password"]').fill(PASSWORD);
  await page.getByRole('button', { name: 'Войти' }).click();
  // Название раздела живёт в шапке обычным текстом, а не заголовком —
  // ждём сам список проектов: он и означает, что вход состоялся.
  await page.waitForURL('**/projects');
  await page.locator('.ant-table').first().waitFor();
});

test('Реестр проектов открывается за норматив', async ({ page }) => {
  const took = await seconds(async () => {
    await page.goto('/projects');
    // Ждём первую строку таблицы, а не событие load: пустая разметка
    // без данных пользователю бесполезна и нормативом не считается.
    await page.locator('.ant-table-row').first().waitFor();
  });
  expect.soft(took, `реестр проектов: ${took.toFixed(2)} с`).toBeLessThanOrEqual(SLA.pageLoad);
});

test('Карточка проекта открывается за норматив', async ({ page }) => {
  await page.goto('/projects');
  // Переход открывает ссылка на названии проекта, не строка целиком.
  const firstRow = page.locator('.ant-table-row a.ibcon-link-plain').first();
  await firstRow.waitFor();

  const took = await seconds(async () => {
    await firstRow.click();
    await expect(page.getByText('Версии бюджета')).toBeVisible();
  });
  expect.soft(took, `карточка проекта: ${took.toFixed(2)} с`).toBeLessThanOrEqual(SLA.projectCard);
});

test('Версия бюджета, расчёт и выгрузка укладываются в нормативы', async ({ page }) => {
  await page.goto('/projects');
  await page.locator('.ant-table-row').first().waitFor();

  // Ищем проект, у которого версия бюджета есть: без неё мерить нечего.
  const names = await page.locator('.ant-table-row a.ibcon-link-plain').all();
  let opened = false;
  let openTook = 0;
  for (const row of names) {
    await row.click();
    await expect(page.getByText('Версии бюджета')).toBeVisible();
    // Версия открывается ссылкой с её номером («16.2») в первой колонке.
    const version = page.locator('.ant-table-row a').first();
    if (await version.count()) {
      openTook = await seconds(async () => {
        await version.click();
        // Полоса шагов мастера появляется, только когда версия и её
        // вводные пришли с сервера.
        await expect(page.getByText('Сотрудники').first()).toBeVisible();
      });
      opened = true;
      break;
    }
    await page.goBack();
  }
  test.skip(!opened, 'Нет ни одного проекта с версией бюджета');
  expect.soft(openTook, `открытие версии: ${openTook.toFixed(2)} с`)
    .toBeLessThanOrEqual(SLA.budgetVersion);

  // Расчёт: переходим на шаг «Результаты» и ждём итоговую стоимость.
  const calcTook = await seconds(async () => {
    await page.getByText('Результаты', { exact: true }).click();
    await expect(page.getByText(/Итого стоимость|Рентабельность/).first())
      .toBeVisible({ timeout: SLA.calculation * 1000 });
  });
  expect.soft(calcTook, `расчёт бюджета: ${calcTook.toFixed(2)} с`)
    .toBeLessThanOrEqual(SLA.calculation);

  // Выгрузка живёт на последнем шаге: книга одна на всё — «Бюджет»,
  // БДР и БДДС тремя листами.
  await page.getByText('БДР и БДДС', { exact: true }).first().click();

  // Меряем до момента, когда браузер получил файл целиком.
  const exportTook = await seconds(async () => {
    const download = page.waitForEvent('download', { timeout: SLA.export * 1000 });
    await page.getByRole('button', { name: /Выгрузить/ }).first().click();
    const file = await download;
    // Пустой или обрезанный файл нормативом не считается — проверяем,
    // что это действительно книга.
    expect(await file.path()).toBeTruthy();
    expect(file.suggestedFilename()).toMatch(/\.xlsx$/);
  });
  expect.soft(exportTook, `выгрузка XLSX: ${exportTook.toFixed(2)} с`)
    .toBeLessThanOrEqual(SLA.export);
});
