import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { message } from 'antd';
import { profileApi } from '../api';
import { extractError } from '../api/client';
import type { FontSize, ThemeMode, UISettings } from '../types';
import { BRAND } from '../theme';

// Персональные настройки интерфейса: размер шрифта, тема и цвета.

/** Цвет уведомлений по умолчанию — тот же фирменный. */
const DEFAULT_NOTICE = BRAND;

const LS_KEY = 'ibcon-ui-settings';

// Базовый кегль в пикселях для каждого размера. Меняется именно РАЗМЕР
// ТЕКСТА, а не масштаб страницы: transform:scale растянул бы и рамки с
// отступами, и вёрстка поехала бы.
export const FONT_SIZE_PX: Record<FontSize, number> = {
  small: 13,
  normal: 14,
  large: 16,
};

export interface UISettingsValue {
  settings: UISettings;
  fontSize: FontSize;
  theme: ThemeMode;
  brandColor: string;
  noticeColor: string;
  /** Сохраняет изменения в учётке; на экране они применяются сразу. */
  update: (patch: UISettings) => void;
  saving: boolean;
}

const Ctx = createContext<UISettingsValue | null>(null);

function readCache(): UISettings {
  try {
    const raw = localStorage.getItem(LS_KEY);
    return raw ? (JSON.parse(raw) as UISettings) : {};
  } catch {
    return {}; // приватный режим — просто начнём с умолчаний
  }
}

function writeCache(s: UISettings) {
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(s));
  } catch { /* нет доступа к хранилищу — настройки всё равно в учётке */ }
}

export function UISettingsProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  // Начинаем с копии из браузера: она применится мгновенно, а ответ
  // сервера её уточнит.
  const [local, setLocal] = useState<UISettings>(readCache);

  const { data: profile } = useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.get,
    // На экране логина профиля нет — запрос просто вернёт ошибку, и
    // останутся умолчания.
    retry: false,
  });

  // Что мы в последний раз ОТПРАВИЛИ на сервер.
  const sent = useRef<UISettings | null>(null);

  useEffect(() => {
    if (!profile) return;
    const fromServer = profile.ui_settings ?? {};
    if (sent.current && !sameSettings(sent.current, fromServer)) return;
    sent.current = null;
    setLocal(fromServer);
    writeCache(fromServer);
  }, [profile]);

  const saveMutation = useMutation({
    mutationFn: (next: UISettings) => profileApi.update({ ui_settings: next }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['profile'] }),
    // Молчаливая неудача выглядела как «настройки ничего не меняют»:
    // на экране новое оформление, а после перезагрузки снова старое.
    onError: (e) => {
      sent.current = null;
      message.error(`Настройки не сохранились: ${extractError(e)}`);
    },
  });

  const value = useMemo<UISettingsValue>(() => ({
    settings: local,
    fontSize: local.font_size ?? 'normal',
    theme: local.theme ?? 'light',
    brandColor: local.brand_color ?? BRAND,
    noticeColor: local.notice_color ?? DEFAULT_NOTICE,
    saving: saveMutation.isPending,
    update: (patch) => {
      const next = { ...local, ...patch };
      // Применяем сразу, не дожидаясь сервера: настройка оформления
      // должна отзываться мгновенно, иначе кажется, что не сработала.
      setLocal(next);
      writeCache(next);
      sent.current = next;
      saveMutation.mutate(next);
    },
  }), [local, saveMutation]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

/** Совпадают ли настройки по значению: ссылки у ответа сервера всегда разные. */
function sameSettings(a: UISettings, b: UISettings): boolean {
  return a.font_size === b.font_size
    && a.theme === b.theme
    && a.brand_color === b.brand_color
    && a.notice_color === b.notice_color;
}

export function useUISettings(): UISettingsValue {
  const v = useContext(Ctx);
  if (!v) {
    // Провайдер стоит в корне приложения; сюда попадаем только если
    // компонент вынесли за него — тогда работаем на умолчаниях.
    return {
      settings: {},
      fontSize: 'normal',
      theme: 'light',
      brandColor: BRAND,
      noticeColor: DEFAULT_NOTICE,
      update: () => {},
      saving: false,
    };
  }
  return v;
}
