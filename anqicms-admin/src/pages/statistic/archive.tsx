import NewContainer from '@/components/NewContainer';
import { getStatisticArchive } from '@/services';
import { ReloadOutlined } from '@ant-design/icons';
import { StatisticCard } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Progress,
  Radio,
  Row,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import dayjs, { Dayjs } from 'dayjs';
import React, { useEffect, useMemo, useState } from 'react';

const { Text } = Typography;

type Dimension = 'year' | 'month';
type Group = 'module' | 'category';

// 后端返回结构（见 provider/statistic_archive.go 的 ArchiveStatResult）。
// counts 的 key 与 periods 一一对应，后端已把缺失周期补 0。
type ArchiveStatItem = {
  id: number;
  name: string;
  unassigned?: boolean;
  module_id: number;
  module_name: string;
  parent_id: number;
  parent_name: string;
  kind: string;
  table_name: string;
  total: number;
  counts: Record<string, number>;
};

type ArchiveStatData = {
  dimension: Dimension;
  group: Group;
  multi_category: boolean;
  start: string;
  end: string;
  periods: string[];
  items: ArchiveStatItem[];
  totals: Record<string, number>;
  total: number;
};

const periodFormat = (dimension: Dimension) =>
  dimension === 'year' ? 'YYYY' : 'YYYY-MM';

// 默认区间与后端保持一致：按月近 12 个月（含当月），按年近 5 年（含当年）。
const defaultRange = (dimension: Dimension): [Dayjs, Dayjs] =>
  dimension === 'year'
    ? [dayjs().subtract(4, 'year'), dayjs()]
    : [dayjs().subtract(11, 'month'), dayjs()];

const StatisticArchive: React.FC = () => {
  const intl = useIntl();
  const [dimension, setDimension] = useState<Dimension>('month');
  const [group, setGroup] = useState<Group>('module');
  const [range, setRange] = useState<[Dayjs, Dayjs]>(defaultRange('month'));
  const [data, setData] = useState<ArchiveStatData>();
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');

  const fetchData = async (
    nextDimension: Dimension = dimension,
    nextGroup: Group = group,
    nextRange: [Dayjs, Dayjs] = range,
  ) => {
    setLoading(true);
    setError('');
    try {
      const res = await getStatisticArchive({
        dimension: nextDimension,
        group: nextGroup,
        start: nextRange[0].format(periodFormat(nextDimension)),
        end: nextRange[1].format(periodFormat(nextDimension)),
      });
      if (res?.code !== 0) {
        // 后端对未知维度/粒度返回 code=-1，这里显式提示，不要静默留白。
        setError(res?.msg || '');
        setData(undefined);
        return;
      }
      setData(res.data);
    } catch (e) {
      console.log('fetch data failed', e);
      setError('');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const onDimensionChange = (next: Dimension) => {
    const nextRange = defaultRange(next);
    setDimension(next);
    setRange(nextRange);
    fetchData(next, group, nextRange);
  };

  const onGroupChange = (next: Group) => {
    setGroup(next);
    fetchData(dimension, next, range);
  };

  const onRangeChange = (values: any) => {
    // RangePicker 设了 allowClear={false}，正常不会拿到空值；
    // 真拿到（受控组件边界）就退回默认区间，避免把 undefined 格式化炸掉。
    const nextRange: [Dayjs, Dayjs] =
      values?.[0] && values?.[1] ? [values[0], values[1]] : defaultRange(dimension);
    setRange(nextRange);
    fetchData(dimension, group, nextRange);
  };

  const items = data?.items || [];
  const periods = data?.periods || [];
  const total = data?.total || 0;
  const isCategory = group === 'category';
  const maxItemTotal = items.length ? items[0].total : 0;

  const itemLabel = (item: ArchiveStatItem) => {
    if (item.unassigned || (!item.name && item.id === 0)) {
      return intl.formatMessage({ id: 'statistic.archive.unassigned' });
    }
    if (!item.name) {
      return `#${item.id}`;
    }
    // 栏目按「父栏目 / 子栏目」展示，避免同名子栏目串台
    return item.parent_name ? `${item.parent_name} / ${item.name}` : item.name;
  };

  const columns = useMemo(() => {
    if (!data) return [];
    const nameColumn = {
      title: intl.formatMessage({
        id: isCategory ? 'statistic.archive.category' : 'statistic.archive.model',
      }),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left' as const,
      width: 240,
      render: (_: any, record: ArchiveStatItem) => (
        <Space size={6}>
          <Text
            strong={record.total > 0}
            type={record.unassigned ? 'secondary' : undefined}
            ellipsis={{ tooltip: itemLabel(record) }}
            style={{ maxWidth: 180 }}
          >
            {itemLabel(record)}
          </Text>
          {!isCategory && record.kind === 'product' && (
            <Tag color="blue">
              {intl.formatMessage({ id: 'statistic.archive.kind.product' })}
            </Tag>
          )}
        </Space>
      ),
    };
    const moduleColumn = isCategory
      ? [
          {
            title: intl.formatMessage({ id: 'statistic.archive.module' }),
            dataIndex: 'module_name',
            key: 'module_name',
            width: 140,
            render: (value: string, record: ArchiveStatItem) =>
              value ? <Text type="secondary">{value}</Text> : <Text type="secondary">-</Text>,
          },
        ]
      : [];
    return [
      nameColumn,
      ...moduleColumn,
      ...data.periods.map((period: string) => ({
        title: period,
        key: period,
        dataIndex: ['counts', period],
        width: 96,
        align: 'right' as const,
        render: (value: number) =>
          value ? <span>{value}</span> : <Text type="secondary">-</Text>,
      })),
      {
        title: intl.formatMessage({ id: 'statistic.archive.sum' }),
        dataIndex: 'total',
        key: 'total',
        width: 110,
        align: 'right' as const,
        fixed: 'right' as const,
        render: (value: number) => <Text strong>{value}</Text>,
      },
    ];
  }, [data, intl, isCategory]);

  const summary = () => {
    if (!data) return null;
    return (
      <Table.Summary fixed>
        <Table.Summary.Row>
          <Table.Summary.Cell index={0}>
            <Text strong>{intl.formatMessage({ id: 'statistic.archive.sum' })}</Text>
          </Table.Summary.Cell>
          {isCategory && (
            <Table.Summary.Cell index={1}>
              <Text type="secondary" />
            </Table.Summary.Cell>
          )}
          {periods.map((period: string, index: number) => (
            <Table.Summary.Cell
              key={period}
              index={index + (isCategory ? 2 : 1)}
              align="right"
            >
              <Text strong>{data.totals?.[period] || 0}</Text>
            </Table.Summary.Cell>
          ))}
          <Table.Summary.Cell
            index={periods.length + (isCategory ? 2 : 1)}
            align="right"
          >
            <Text strong type="danger">
              {total}
            </Text>
          </Table.Summary.Cell>
        </Table.Summary.Row>
      </Table.Summary>
    );
  };

  const unitLabel = intl.formatMessage({
    id: dimension === 'year' ? 'statistic.archive.unit.year' : 'statistic.archive.unit.month',
  });

  return (
    <NewContainer onTabChange={() => fetchData()}>
      <Card bodyStyle={{ paddingBottom: 12 }}>
        <Space wrap size={16}>
          <Space size={8}>
            <Text type="secondary">
              {intl.formatMessage({ id: 'statistic.archive.dimension' })}
            </Text>
            <Radio.Group
              value={dimension}
              onChange={(e) => onDimensionChange(e.target.value)}
              optionType="button"
              buttonStyle="solid"
            >
              <Radio.Button value="month">
                {intl.formatMessage({ id: 'statistic.archive.dimension.month' })}
              </Radio.Button>
              <Radio.Button value="year">
                {intl.formatMessage({ id: 'statistic.archive.dimension.year' })}
              </Radio.Button>
            </Radio.Group>
          </Space>
          <Space size={8}>
            <Text type="secondary">
              {intl.formatMessage({ id: 'statistic.archive.group' })}
            </Text>
            <Radio.Group
              value={group}
              onChange={(e) => onGroupChange(e.target.value)}
              optionType="button"
              buttonStyle="solid"
            >
              <Radio.Button value="module">
                {intl.formatMessage({ id: 'statistic.archive.group.module' })}
              </Radio.Button>
              <Radio.Button value="category">
                {intl.formatMessage({ id: 'statistic.archive.group.category' })}
              </Radio.Button>
            </Radio.Group>
          </Space>
          <DatePicker.RangePicker
            picker={dimension}
            allowClear={false}
            value={range}
            format={periodFormat(dimension)}
            onChange={onRangeChange}
          />
          <Button icon={<ReloadOutlined />} loading={loading} onClick={() => fetchData()}>
            {intl.formatMessage({ id: 'statistic.archive.refresh' })}
          </Button>
          <Text type="secondary">
            {intl.formatMessage({ id: 'statistic.archive.scope' })}
          </Text>
          {data && (
            <Text type="secondary">
              {intl.formatMessage({ id: 'statistic.archive.range' })}: {data.start} ~ {data.end}
            </Text>
          )}
        </Space>
      </Card>

      {error && <Alert type="error" showIcon style={{ marginTop: 16 }} message={error} />}

      {isCategory && data?.multi_category && (
        <Alert
          type="info"
          showIcon
          style={{ marginTop: 16 }}
          message={intl.formatMessage({ id: 'statistic.archive.multi-category-tip' })}
        />
      )}

      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col xs={24} md={8}>
          <StatisticCard
            loading={loading}
            statistic={{
              title: intl.formatMessage({ id: 'statistic.archive.total' }),
              value: total,
              suffix: intl.formatMessage({ id: 'statistic.archive.unit.doc' }),
            }}
          />
        </Col>
        <Col xs={24} md={8}>
          <StatisticCard
            loading={loading}
            statistic={{
              title: intl.formatMessage({
                id: isCategory
                  ? 'statistic.archive.category-count'
                  : 'statistic.archive.module-count',
              }),
              value: items.filter((item) => item.total > 0).length,
              suffix: `/ ${items.length}`,
            }}
          />
        </Col>
        <Col xs={24} md={8}>
          <StatisticCard
            loading={loading}
            statistic={{
              title: intl.formatMessage({ id: 'statistic.archive.period-count' }),
              value: periods.length,
              suffix: unitLabel,
            }}
          />
        </Col>
      </Row>

      {!loading && items.length === 0 && (
        <Card style={{ marginTop: 16 }}>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={intl.formatMessage({
              id: isCategory ? 'statistic.archive.no-category' : 'statistic.archive.no-module',
            })}
          />
        </Card>
      )}

      {items.length > 0 && total === 0 && (
        <Card style={{ marginTop: 16 }}>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={intl.formatMessage({ id: 'statistic.archive.empty' })}
          />
        </Card>
      )}

      {total > 0 && (
        <>
          <Card
            style={{ marginTop: 16 }}
            title={intl.formatMessage({
              id: isCategory
                ? 'statistic.archive.distribution-category'
                : 'statistic.archive.distribution',
            })}
            loading={loading}
          >
            {items
              .filter((item) => item.total > 0)
              .map((item) => (
                <div key={`${item.id}-${item.name}`} style={{ marginBottom: 12 }}>
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      marginBottom: 4,
                    }}
                  >
                    <Text ellipsis={{ tooltip: itemLabel(item) }} style={{ maxWidth: '70%' }}>
                      {itemLabel(item)}
                    </Text>
                    <Text type="secondary">
                      {item.total} ({((item.total / total) * 100).toFixed(1)}%)
                    </Text>
                  </div>
                  <Progress
                    percent={Math.round((item.total / (maxItemTotal || 1)) * 100)}
                    showInfo={false}
                    strokeColor="#1890ff"
                  />
                </div>
              ))}
          </Card>

          <Card
            style={{ marginTop: 16 }}
            title={intl.formatMessage({ id: 'statistic.archive.detail' })}
            loading={loading}
          >
            <Table
              rowKey={(record: ArchiveStatItem) => `${record.id}-${record.name}`}
              size="small"
              bordered
              columns={columns as any}
              dataSource={items}
              pagination={false}
              scroll={{ x: 'max-content', y: 560 }}
              summary={summary}
            />
          </Card>
        </>
      )}
    </NewContainer>
  );
};

export default StatisticArchive;
