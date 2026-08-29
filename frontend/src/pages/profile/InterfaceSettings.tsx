import { ColorPicker, Segmented, Space, Typography } from 'antd';
import type { Color } from 'antd/es/color-picker';
import { useUISettings } from '../../store/uiSettings';
import { FONT_SIZE_LABELS, FONT_SIZES } from '../../types';
import type { FontSize, ThemeMode } from '../../types';
import { BRAND } from '../../theme';

const { Text } = Typography;

/**
 * Настройки интерфейса: размер шрифта, тема и цвета.
 *
 * Применяются сразу, без кнопки «Сохранить»: настройка оформления должна
 * отзываться мгновенно, иначе кажется, что не сработала. В учётку они
 * уходят тем же действием (см. store/uiSettings).
 */
export default function InterfaceSettings() {
  const { fontSize, theme, brandColor, noticeColor, update } = useUISettings();

  const row = (label: string, hint: string, control: React.ReactNode) => (
    <div style={{ marginBottom: 16 }}>
      <div style={{ marginBottom: 6 }}>
        <Text style={{ display: 'block' }}>{label}</Text>
        <Text type="secondary" style={{ fontSize: 12 }}>{hint}</Text>
      </div>
      {control}
    </div>
  );

  return (
    <div>
      {row(
        'Размер шрифта',
        'Меняется размер текста, а не масштаб страницы: отступы и рамки остаются на месте.',
        <Segmented
          value={fontSize}
          onChange={(v) => update({ font_size: v as FontSize })}
          options={Object.values(FONT_SIZES).map(size => ({
            value: size,
            label: FONT_SIZE_LABELS[size],
          }))}
        />,
      )}

      {row(
        'Тема',
        'Тёмная тема меняет фон и цвет текста; фирменный цвет остаётся вашим.',
        <Segmented
          value={theme}
          onChange={(v) => update({ theme: v as ThemeMode })}
          options={[
            { value: 'light', label: 'Светлая' },
            { value: 'dark', label: 'Тёмная' },
          ]}
        />,
      )}

      {row(
        'Фирменный цвет',
        'Им красится всё, что было синим: шапка, кнопки, выделение в меню.',
        <Space>
          <ColorPicker
            value={brandColor}
            onChangeComplete={(c: Color) => update({ brand_color: c.toHexString() })}
            showText
          />
          {brandColor.toLowerCase() !== BRAND.toLowerCase() && (
            <a onClick={() => update({ brand_color: BRAND })}>Вернуть фирменный</a>
          )}
        </Space>,
      )}

      {row(
        'Цвет уведомлений',
        'Всплывающие сообщения и напоминания. Их замечают краем глаза — цвет может отличаться от фирменного.',
        <Space>
          <ColorPicker
            value={noticeColor}
            onChangeComplete={(c: Color) => update({ notice_color: c.toHexString() })}
            showText
          />
          {noticeColor.toLowerCase() !== brandColor.toLowerCase() && (
            <a onClick={() => update({ notice_color: brandColor })}>Как фирменный</a>
          )}
        </Space>,
      )}
    </div>
  );
}
