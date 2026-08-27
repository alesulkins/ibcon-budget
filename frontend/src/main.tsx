import React from 'react';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ConfigProvider } from 'antd';
import ruRU from 'antd/locale/ru_RU';
import dayjs from 'dayjs';
import 'dayjs/locale/ru';
import App from './App';
import {
  BRAND, BRAND_WHITE, PAGE_BG, FONT_UI, RADIUS, RADIUS_LG, RADIUS_SM,
  TEXT, TEXT_SOFT, LINE,
} from './theme';
import './index.css';

dayjs.locale('ru');

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ConfigProvider
        locale={ruRU}
        theme={{
          // cssVar переводит antd на CSS-переменные: стили считаются один
          // раз, а не пересобираются рантайм-движком на каждую новую
          // комбинацию компонентов. hashed: false убирает хеш-классы —
          // меньше работы стилевому движку и чище инспектор.
          cssVar: { key: 'ibcon' },
          hashed: false,
          token: {
            colorPrimary: BRAND,
            // Поля ввода и всплывающие поверхности — фирменный белый.
            // Плиты карточек и таблиц убраны в index.css: выбранное
            // оформление ставит содержимое прямо на рабочую область.
            colorBgContainer: BRAND_WHITE,
            colorBgElevated: BRAND_WHITE,
            // Совпадает с --ibcon-bg в index.css: иначе на краях страницы
            // виден стык двух почти одинаковых фонов.
            colorBgLayout: PAGE_BG,
            // Текст и линии — из пульта оформления. Серые по умолчанию у
            // antd нейтральные и рядом с фирменным цветом выглядят
            // грязноватыми; эти уведены в ту же сине-зелёную сторону.
            colorText: TEXT,
            colorTextHeading: TEXT,
            colorTextSecondary: TEXT_SOFT,
            colorTextDescription: TEXT_SOFT,
            colorTextPlaceholder: TEXT_SOFT,
            colorBorder: LINE,
            colorBorderSecondary: LINE,
            colorSplit: LINE,
            fontFamily: FONT_UI,
            borderRadius: RADIUS,
            borderRadiusLG: RADIUS_LG,
            borderRadiusSM: RADIUS_SM,
            fontSize: 14,
          },
        }}
      >
        <App />
      </ConfigProvider>
    </QueryClientProvider>
  </StrictMode>,
);
