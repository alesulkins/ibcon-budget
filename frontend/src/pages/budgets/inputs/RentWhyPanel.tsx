import { useCallback, useRef, useState } from 'react';
import { Button, Collapse, Typography } from 'antd';
import { QuestionCircleOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { marketApi } from '../../../api';

const { Paragraph } = Typography;

// «Почему такая цена» — раскрывающаяся панель под результатом. Свёрнута по
// умолчанию: основное окно остаётся прежним, а пояснения читают, только
// когда цифра вызвала вопрос.
export default function RentWhyPanel() {
  const [open, setOpen] = useState(false);
  const panel = useRef<HTMLDivElement>(null);

  const { data: blocks } = useQuery({
    queryKey: ['market-methodology'],
    queryFn: marketApi.methodology,
    // Текст методики не меняется от расчёта к расчёту.
    staleTime: Infinity,
    enabled: open,
  });

  // После раскрытия панель подводится к глазам плавной прокруткой: она
  // появляется НИЖЕ таблиц с примерами, и человек её просто не находил.
  const toggle = useCallback(() => {
    setOpen(v => {
      if (!v) {
        setTimeout(() => {
          panel.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }, 60);
      }
      return !v;
    });
  }, []);

  return (
    <div style={{ marginTop: 12 }}>
      <Button
        type="text"
        icon={<QuestionCircleOutlined />}
        onClick={toggle}
        style={{ paddingLeft: 0 }}
      >
        Почему такая цена?
      </Button>

      {open && (
        <div ref={panel} className="ibcon-glass" style={{ padding: 14, marginTop: 8 }}>
          <Collapse
            ghost
            size="small"
            items={(blocks ?? []).map((b, i) => ({
              key: String(i),
              label: b.title,
              children: <Paragraph style={{ marginBottom: 0 }}>{b.text}</Paragraph>,
            }))}
          />
        </div>
      )}
    </div>
  );
}
