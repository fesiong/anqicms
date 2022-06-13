import { InfoCircleOutlined } from '@ant-design/icons';
import { Col, Row, Space, Statistic, Tooltip } from 'antd';

import { ChartCard, Field } from './Charts';

const topColResponsiveProps = {
  xs: 24,
  sm: 12,
  md: 12,
  lg: 12,
  xl: 6,
  style: { marginBottom: 24 },
};

const StatisticsRow = ({ loading, data }: { loading: boolean; data: any }) => (

  <Row gutter={24}>
    <Col {...topColResponsiveProps}>
      <ChartCard
        bordered={false}
        title="文档量"
        action={
          <Tooltip title="包括各个模型文档">
            <InfoCircleOutlined />
          </Tooltip>
        }
        total={data.archive_count?.total}
        footer={<Space>
          <Field label="上周" value={data.archive_count?.last_week} />
          <Field label="待发布" value={data.archive_count?.un_release} />
          <Field label="今日" value={data.archive_count?.today} />
        </Space>}
        contentHeight={46}
      >
      </ChartCard>
    </Col>

    <Col {...topColResponsiveProps}>
      <ChartCard
        bordered={false}
        loading={loading}
        title="一周访问量"
        action={
          <Tooltip title="网页访问数据">
            <InfoCircleOutlined />
          </Tooltip>
        }
        total={data.traffic_count?.total}
        footer={<Field label="今日访问" value={data.traffic_count?.today} />}
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
        title="一周蜘蛛访问"
        action={
          <Tooltip title="蜘蛛访问记录">
            <InfoCircleOutlined />
          </Tooltip>
        }
        total={data.spider_count?.total}
        footer={<Field label="今日访问" value={data.spider_count?.today} />}
        contentHeight={46}
      >
        {/* <TinyColumn xField="x" height={46} forceFit yField="y" data={visitData} /> */}
      </ChartCard>
    </Col>
    <Col {...topColResponsiveProps}>
      <ChartCard
        bordered={false}
        loading={loading}
        title="收录情况"
        action={
          <Tooltip title="搜索引擎收录情况">
            <InfoCircleOutlined />
          </Tooltip>
        }
        contentHeight={82}
      >
        <Row style={{textAlign: 'center'}}>
          <Col flex={1}>
          <Statistic title='百度' value={data.include_count?.baidu_count} />
          </Col>
          <Col flex={1}>
          <Statistic title='搜狗' value={data.include_count?.sogou_count} />
          </Col>
          <Col flex={1}>
          <Statistic title='搜搜' value={data.include_count?.so_count} />
          </Col>
          {/* <Col flex={1}>
          <Statistic title='必应' value={data.include_count?.bing_count} />
          </Col>
          <Col flex={1}>
          <Statistic title='谷歌' value={data.include_count?.google_count} />
          </Col> */}
        </Row>
        </ChartCard>
    </Col>
  </Row>
);

export default StatisticsRow;
