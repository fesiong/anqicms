import { InfoCircleOutlined } from '@ant-design/icons';
import { Col, Row, Space, Statistic, Tag, Tooltip } from 'antd';

import { FormattedMessage, Link, useIntl } from '@umijs/max';
import { ChartCard, Field } from './Charts';

const topColResponsiveProps = {
  xs: 24,
  sm: 12,
  md: 12,
  lg: 12,
  xl: 6,
  style: { marginBottom: 24 },
};

const StatisticsRow = ({
  loading,
  data,
  exact,
  getExactCount,
}: {
  loading: boolean;
  data: any;
  exact: boolean;
  getExactCount: () => void;
}) => {
  const intl = useIntl();
  return (
    <Row gutter={24}>
      <Col {...topColResponsiveProps}>
        <ChartCard
          bordered={false}
          title={intl.formatMessage({
            id: 'dashboard.component.archive.count',
          })}
          extra={
            !exact && (
              <Tooltip
                title={intl.formatMessage({
                  id: 'content.count.tips.tooltip',
                })}
              >
                <Tag className="link" color="orange" onClick={getExactCount}>
                  <FormattedMessage id="content.count.tips" />
                </Tag>
              </Tooltip>
            )
          }
          action={
            <Tooltip
              title={intl.formatMessage({
                id: 'dashboard.component.archive.count.tips',
              })}
            >
              <InfoCircleOutlined />
            </Tooltip>
          }
          total={
            (data.archive_count?.total || 0) +
            '/' +
            (data.archive_count?.un_release || 0) +
            '/' +
            (data.archive_count?.draft || 0)
          }
          footer={
            <Space>
              <Field
                label={intl.formatMessage({
                  id: 'dashboard.component.lastweek',
                })}
                value={data.archive_count?.last_week || 0}
              />
              <Field
                label={intl.formatMessage({ id: 'dashboard.component.today' })}
                value={data.archive_count?.today || 0}
              />
              <Link to={'/product/index?status=plan'}>
                <Field
                  label={intl.formatMessage({
                    id: 'dashboard.component.unrelease',
                  })}
                  value={data.archive_count?.un_release || 0}
                />
              </Link>
              <Link to={'/product/index?status=draft'}>
                <Field
                  label={intl.formatMessage({
                    id: 'dashboard.component.draft',
                  })}
                  value={data.archive_count?.draft || 0}
                />
              </Link>
            </Space>
          }
          contentHeight={46}
        ></ChartCard>
      </Col>

      <Col {...topColResponsiveProps}>
        <ChartCard
          bordered={false}
          loading={loading}
          title={intl.formatMessage({ id: 'dashboard.component.week-traffic' })}
          action={
            <Tooltip
              title={intl.formatMessage({
                id: 'dashboard.component.traffic-description',
              })}
            >
              <InfoCircleOutlined />
            </Tooltip>
          }
          total={data.traffic_count?.total}
          footer={
            <Field
              label={intl.formatMessage({
                id: 'dashboard.component.today-visit',
              })}
              value={data.traffic_count?.today}
            />
          }
          contentHeight={46}
        >
          {/* <TinyArea
          color="#975FE4"
          xField="x"
          height={46}
          forceFit
          yField="y"
          smooth
          data={visitData}
        /> */}
        </ChartCard>
      </Col>
      <Col {...topColResponsiveProps}>
        <ChartCard
          bordered={false}
          loading={loading}
          title={intl.formatMessage({ id: 'dashboard.component.week-spider' })}
          action={
            <Tooltip
              title={intl.formatMessage({
                id: 'dashboard.component.spider-description',
              })}
            >
              <InfoCircleOutlined />
            </Tooltip>
          }
          total={data.spider_count?.total}
          footer={
            <Field
              label={intl.formatMessage({
                id: 'dashboard.component.today-visit',
              })}
              value={data.spider_count?.today}
            />
          }
          contentHeight={46}
        >
          {/* <TinyColumn xField="x" height={46} forceFit yField="y" data={visitData} /> */}
        </ChartCard>
      </Col>
      <Col {...topColResponsiveProps}>
        <ChartCard
          bordered={false}
          loading={loading}
          title={intl.formatMessage({ id: 'dashboard.component.indexing' })}
          action={
            <Tooltip
              title={intl.formatMessage({
                id: 'dashboard.component.indexing-description',
              })}
            >
              <InfoCircleOutlined />
            </Tooltip>
          }
          contentHeight={82}
        >
          <Row style={{ textAlign: 'center' }}>
            <Col flex={1}>
              <Statistic
                title={intl.formatMessage({ id: 'dashboard.component.baidu' })}
                value={data.include_count?.baidu_count}
              />
            </Col>
            <Col flex={1}>
              <Statistic
                title={intl.formatMessage({ id: 'dashboard.component.sogou' })}
                value={data.include_count?.sogou_count}
              />
            </Col>
            {/* <Col flex={1}>
              <Statistic title={intl.formatMessage({ id: 'dashboard.component.soso' })} value={data.include_count?.so_count} />
            </Col> */}
            <Col flex={1}>
              <Statistic
                title={intl.formatMessage({ id: 'dashboard.component.bing' })}
                value={data.include_count?.bing_count}
              />
            </Col>
            <Col flex={1}>
              <Statistic
                title={intl.formatMessage({ id: 'dashboard.component.google' })}
                value={data.include_count?.google_count}
              />
            </Col>
          </Row>
        </ChartCard>
      </Col>
    </Row>
  );
};

export default StatisticsRow;
