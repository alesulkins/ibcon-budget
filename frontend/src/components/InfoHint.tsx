import type { ReactNode } from 'react';
import { Tooltip, Space } from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';

// Иконка «i» с пояснением по наведению — эталон взят с экрана «Премии».
// Заменяет постоянные абзацы-пояснения под блоками.
export default function InfoHint({ text }: { text: ReactNode }) {
  return (
    <Tooltip title={text} styles={{ root: { maxWidth: 420 } }}>
      <InfoCircleOutlined style={{ color: '#8c8c8c', fontSize: 14 }} />
    </Tooltip>
  );
}

/** Заголовок карточки с иконкой пояснения справа. */
export function titleWithHint(title: ReactNode, hint: ReactNode) {
  return (
    <Space size={6}>
      <span>{title}</span>
      <InfoHint text={hint} />
    </Space>
  );
}
