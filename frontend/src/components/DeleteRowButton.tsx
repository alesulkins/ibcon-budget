import { Button, Popconfirm } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';

interface Props {
  onConfirm: () => void;
  /**
   * Что именно удаляется: «Удалить строку?», «Удалить сотрудника?».
   * Введённые в строке данные восстановить нечем, поэтому спрашиваем
   * всегда — даже когда строка выглядит пустой.
   */
  title?: string;
  /** `text` — кнопка без рамки, для таблиц внутри форм ввода. */
  variant?: 'text' | 'default';
}

/**
 * Кнопка удаления строки с подтверждением. Одна на все таблицы, чтобы
 * вопрос, кнопки и вид иконки нигде не разъезжались.
 */
export default function DeleteRowButton({
  onConfirm, title = 'Удалить строку?', variant = 'text',
}: Props) {
  return (
    <Popconfirm
      title={title}
      okText="Да"
      cancelText="Нет"
      onConfirm={onConfirm}
    >
      {/* Красный убран: удаление строки формы — обычное действие, а не
          поломка. Знак и кнопка подтверждения идут выбранным в
          настройках цветом, как и остальное оформление. */}
      <Button
        size="small"
        type={variant}
        icon={<DeleteOutlined />}
        style={{ color: 'var(--ibcon-brand)' }}
      />
    </Popconfirm>
  );
}
