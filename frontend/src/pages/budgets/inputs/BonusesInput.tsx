import { useCallback, useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, InputNumber,
  Select, Space, Typography,
} from 'antd';
import { PlusOutlined, EditOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import type { ColumnsType } from 'antd/es/table';
import { budgetsApi } from '../../../api';
import {
  BONUS_KIND_BUILDER_DAY, BONUS_KIND_NEW_YEAR, BONUS_KIND_OTHER,
} from '../../../types';
import type { BonusType, InputBonuses } from '../../../types';
import DeleteRowButton from '../../../components/DeleteRowButton';
import { titleWithHint } from '../../../components/InfoHint';
import { useAutosave } from '../../../hooks/useAutosave';

const { Text } = Typography;

const MONTH_NAMES = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'];

/** Виды премий и их значения по умолчанию (совпадают с бэкендом). */
const BONUS_KINDS = [
  { value: BONUS_KIND_BUILDER_DAY, label: 'День строителя', month: 8, pct: 20 },
  { value: BONUS_KIND_NEW_YEAR, label: 'Новый год', month: 12, pct: 50 },
  { value: BONUS_KIND_OTHER, label: 'Другое', month: undefined, pct: undefined },
];

function kindLabel(kind: string): string {
  return BONUS_KINDS.find(k => k.value === kind)?.label ?? kind;
}

interface Props {
  versionId: number;
  duration: number;
  startDate: string;
  readonly?: boolean;
}

interface BonusFormValues {
  kind: string;
  name: string;
  month_num: number;
  pct_of_salary: number;
}

export default function BonusesInput({ versionId, readonly }: Props) {
  const [bonusTypes, setBonusTypes] = useState<BonusType[]>([]);
  const [showAdd, setShowAdd] = useState(false);
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [form] = Form.useForm<BonusFormValues>();

  const { data: savedBonuses, isSuccess } = useQuery({
    queryKey: ['budget-input', versionId, 'bonuses'],
    queryFn: () => budgetsApi.getInput<InputBonuses>(versionId, 'bonuses'),
  });

  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    if (!isSuccess) return;
    if (savedBonuses?.bonus_types) {
      setBonusTypes(savedBonuses.bonus_types);
    }
    setHydrated(true);
  }, [savedBonuses, isSuccess]);

  // Передаём только виды премий. Суммы по сотрудникам считает бэкенд
  // (calcBonuses): процент от проиндексированного оклада в нужный месяц.
  const save = useCallback(
    (types: BonusType[]) =>
      budgetsApi.saveInput(versionId, 'bonuses', { bonus_types: types } as InputBonuses),
    [versionId],
  );

  useAutosave({ data: bonusTypes, ready: hydrated, save, enabled: !readonly });

  function addOrEditBonus(vals: BonusFormValues) {
    const bt: BonusType = {
      kind: vals.kind,
      name: vals.name,
      month_num: vals.month_num,
      pct_of_salary: vals.pct_of_salary,
    };
    if (editingIdx !== null) {
      const next = [...bonusTypes];
      next[editingIdx] = bt;
      setBonusTypes(next);
    } else {
      setBonusTypes([...bonusTypes, bt]);
    }
    setShowAdd(false);
    form.resetFields();
    setEditingIdx(null);
  }

  /** Подставляет значения по умолчанию при выборе стандартного вида премии. */
  function onKindChange(kind: string) {
    const preset = BONUS_KINDS.find(k => k.value === kind);
    if (!preset) return;
    form.setFieldsValue({
      name: preset.value === BONUS_KIND_OTHER ? '' : preset.label,
      month_num: preset.month,
      pct_of_salary: preset.pct,
    });
  }

  function removeBonus(idx: number) {
    setBonusTypes(bonusTypes.filter((_, i) => i !== idx));
  }

  const columns: ColumnsType<BonusType> = [
    { title: '№', render: (_, __, i) => i + 1, width: 40 },
    { title: 'Вид', dataIndex: 'kind', width: 150, render: (v: string) => kindLabel(v) },
    { title: 'Наименование', dataIndex: 'name' },
    {
      title: 'Месяц начисления',
      dataIndex: 'month_num',
      width: 160,
      render: (v: number) => `${v} — ${MONTH_NAMES[v - 1] ?? ''}`,
    },
    {
      title: '% от оклада',
      dataIndex: 'pct_of_salary',
      width: 110,
      align: 'right',
      render: (v: number) => `${v} %`,
    },
    {
      title: '',
      key: 'actions',
      width: 80,
      // Иконки те же, что на листе «Сотрудники» — эталон для всех листов.
      render: (_, bt, idx) => !readonly && (
        <Space size={4}>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => {
              form.setFieldsValue(bt as BonusFormValues);
              setEditingIdx(idx);
              setShowAdd(true);
            }}
          />
          <DeleteRowButton
            variant="default"
            title="Удалить вид премии?"
            onConfirm={() => removeBonus(idx)}
          />
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card
        title={titleWithHint(
          'Виды премий',
          'Премии и компенсации при увольнении рассчитываются автоматически. '
          + 'Премия начисляется каждому сотруднику, который в этот месяц '
          + 'является сотрудником. Дополнительно начисляется компенсация '
          + 'при увольнении.',
        )}
        size="small"
        style={{ marginBottom: 16 }}
        extra={
          !readonly && (
            <Button
              size="small"
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => { form.resetFields(); setEditingIdx(null); setShowAdd(true); }}
            >
              Добавить вид премии
            </Button>
          )
        }
      >
        <Table
          rowKey={(_, i) => i!}
          columns={columns}
          dataSource={bonusTypes}
          size="small"
          pagination={false}
          // Пока строк нет, шапка таблицы не нужна — только подсказка.
          showHeader={bonusTypes.length > 0}
          locale={{
            emptyText: (
              <Text type="secondary" style={{ fontSize: 12 }}>Видов премий нет</Text>
            ),
          }}
        />
        <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
          Если две премии выпадают одному сотруднику на один месяц, они суммируются —
          система запросит подтверждение.
        </Text>
      </Card>

      <Modal
        title={editingIdx !== null ? 'Редактировать вид премии' : 'Добавить вид премии'}
        open={showAdd}
        onCancel={() => { setShowAdd(false); form.resetFields(); setEditingIdx(null); }}
        onOk={() => form.submit()}
        okText="Сохранить"
        cancelText="Отмена"
        width={420}
      >
        <Form form={form} layout="vertical" onFinish={addOrEditBonus}>
          <Form.Item
            name="kind"
            label="Вид премии"
            rules={[{ required: true, message: 'Выберите вид премии' }]}
          >
            <Select
              options={BONUS_KINDS.map(k => ({ value: k.value, label: k.label }))}
              onChange={onKindChange}
              placeholder="Выберите вид"
            />
          </Form.Item>
          <Form.Item
            name="name"
            label="Наименование"
            rules={[{ required: true, message: 'Укажите название' }]}
          >
            <Input placeholder="День строителя" />
          </Form.Item>
          <Form.Item
            name="month_num"
            label="Месяц начисления"
            rules={[{ required: true, message: 'Укажите месяц' }]}
          >
            <Select
              options={MONTH_NAMES.map((name, i) => ({ value: i + 1, label: `${i + 1} — ${name}` }))}
              placeholder="Выберите месяц"
            />
          </Form.Item>
          <Form.Item
            name="pct_of_salary"
            label="% от оклада"
            rules={[{ required: true, message: 'Укажите %' }]}
            help="По умолчанию: День строителя — 20 %, Новый год — 50 %"
          >
            <InputNumber style={{ width: '100%' }} min={0} max={500} addonAfter="%" placeholder="20" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
