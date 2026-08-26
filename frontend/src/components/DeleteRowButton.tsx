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
      okButtonProps={{ danger: true }}
      onConfirm={onConfirm}
    >
      <Button size="small" type={variant} danger icon={<DeleteOutlined />} />
    </Popconfirm>
  );
}
