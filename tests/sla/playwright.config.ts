import { defineConfig, devices } from '@playwright/test';

// Minimal typing for `process.env` to satisfy TypeScript without
// requiring @types/node in this repo.
declare const process: { env: { [key: string]: string | undefined } };

// Браузерная проверка нормативов времени отклика. Меряет то, что видит
// пользователь: не ответ сервера, а момент, когда на экране появились
// данные.
const yandex = process.env.YANDEX_PATH;

export default defineConfig({
  testDir: '.',
  // Норматив на расчёт — 2 минуты; проверка должна успеть его дождаться,
  // чтобы отличить «медленно» от «не работает».
  timeout: 180_000,
  expect: { timeout: 30_000 },
  workers: Number(process.env.USERS ?? 10),
  fullyParallel: true,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:5173',
    trace: 'retain-on-failure',
    // Проверяем скорость, а не оформление: снимки экрана только на
    // упавших проверках.
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'Chrome', use: { ...devices['Desktop Chrome'], channel: 'chrome' } },
    { name: 'Edge', use: { ...devices['Desktop Chrome'], channel: 'msedge' } },
    ...(yandex
      ? [{
          name: 'Yandex',
          use: {
            ...devices['Desktop Chrome'],
            launchOptions: { executablePath: yandex },
          },
        }]
      : []),
  ],
});
