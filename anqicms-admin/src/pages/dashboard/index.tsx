import { brand } from '@/oem/brand';
import {
  checkVersion,
  getDashboardInfo,
  getStatisticOrder,
  getStatisticSpider,
  getStatisticSummary,
  getStatisticTraffic,
} from '@/services';
import { contentEditPath, moduleListPath } from '@/utils/contentRoutes';
import { isShopMode } from '@/utils/shopMode';
import { getStore, setStore } from '@/utils/store';
import { Line } from '@ant-design/plots';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl, useModel } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  Row,
  Space,
  Statistic,
  Tabs,
  Tag,
  Tooltip,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { Suspense, useEffect, useState } from 'react';
import Guide from './components/guide';
import StatisticsRow from './components/statistics';
import './index.less';

const { TabPane } = Tabs;

const Dashboard: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const [data, setData] = useState<any>({});
  const [ordersData, setOrdersData] = useState<any[]>([]);
  const [spiderData, setSpiderData] = useState<any[]>([]);
  const [trafficData, setTrafficData] = useState<any[]>([]);
  const [infoData, setInfoData] = useState<any>({});
  const [newVersion, setNewVersion] = useState<any>(null);
  const [showGuide, setShowGuide] = useState<boolean>(false);
  const [exact, setExact] = useState<boolean>(true);
  const intl = useIntl();

  // 订单统计只在商城模式下有意义：普通内容模式既隐藏该选项卡，也不再请求订单数据。
  // initialState.system 在首屏渲染前已由 getInitialState 取回，这里不会出现先显示后隐藏的闪烁。
  const shopMode = isShopMode(initialState?.system);

  const getSummary = (ex: boolean) => {
    const hide = message.loading(
      {
        content: 'loading...',
        key: 'loading',
      },
      0,
    );
    getStatisticSummary({
      exact: ex,
    })
      .then((res) => {
        setData(res.data || {});
        setExact(res.data?.exact || false);
        let needShow = res.data?.show_guide || false;
        // 读取localStorage
        let hasClose = getStore('close_guide') || false;
        if (hasClose) {
          needShow = false;
        }
        setShowGuide(needShow);
      })
      .catch(() => {})
      .finally(() => {
        hide();
      });
  };

  const getSetting = async () => {
    getSummary(false);
    if (shopMode) {
      getStatisticOrder()
        .then((res) => {
          setOrdersData(res.data || []);
        })
        .catch(() => {});
    }
    getStatisticTraffic()
      .then((res) => {
        setTrafficData(res.data || []);
      })
      .catch(() => {});
    getStatisticSpider()
      .then((res) => {
        setSpiderData(res.data || []);
      })
      .catch(() => {});
    getDashboardInfo()
      .then((res) => {
        setInfoData(res.data || []);
      })
      .catch(() => {});
    checkVersion()
      .then((res) => {
        setNewVersion(res.data || null);
      })
      .catch(() => {});
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleJump = (str: string) => {
    history.push(str);
  };

  const handlePreview = async () => {
    let baseUrl = '';
    if (!initialState?.system) {
      const system = await initialState?.fetchSystemSetting?.();
      if (system) {
        await setInitialState((s) => ({
          ...s,
          system: system,
        }));
      }
      baseUrl = system?.base_url || '';
    } else {
      baseUrl = initialState?.system?.base_url || '';
    }
    window.open(baseUrl);
    return;
  };

  const handleCloseGuide = () => {
    setStore('close_guide', true);
    setShowGuide(false);
  };

  const ordersConfig = {
    data: ordersData,
    //padding: 'auto',
    xField: 'date',
    yField: 'value',
    seriesField: 'label',
    colorField: 'label',
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  const trafficConfig = {
    data: trafficData,
    //padding: 'auto',
    xField: 'date',
    yField: 'value',
    seriesField: 'label',
    colorField: 'label',
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  const spiderConfig = {
    data: spiderData,
    //padding: 'auto',
    xField: 'date',
    yField: 'value',
    seriesField: 'label',
    colorField: 'label',
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  return (
    <PageContainer extra={<div></div>}>
      {showGuide && (
        <Suspense fallback={null}>
          <Guide
            shopMode={shopMode}
            onJump={handleJump}
            onPreview={handlePreview}
            onClose={handleCloseGuide}
          />
        </Suspense>
      )}
      <Suspense fallback={null}>
        <StatisticsRow
          loading={false}
          data={data}
          exact={exact}
          getExactCount={() => {
            getSummary(true);
          }}
        />
      </Suspense>
      <Row gutter={20}>
        <Col sm={18} xs={24}>
          <Card
            title={intl.formatMessage({ id: 'dashboard.quick' })}
            extra={
              <Button onClick={() => handleJump(contentEditPath())}>
                <FormattedMessage id="dashboard.guide.step.publish" />
              </Button>
            }
          >
            <Row gutter={16}>
              {data.archive_counts?.map((item: any, index: number) => (
                <Col
                  flex={1}
                  key={index}
                  onClick={() => {
                    // 逐模型跳转按 kind + 站点模式分流，不再一律写死 /product/index
                    handleJump(moduleListPath(item));
                  }}
                >
                  <Statistic
                    className="link"
                    title={item.name}
                    value={item.total}
                  />
                </Col>
              ))}
              <Col
                flex={1}
                onClick={() => {
                  handleJump('/archive/category');
                }}
              >
                <Statistic
                  className="link"
                  title={intl.formatMessage({ id: 'menu.archive.category' })}
                  value={data.category_count}
                />
              </Col>
              <Col
                flex={1}
                onClick={() => {
                  handleJump('/plugin/system/friendlink');
                }}
              >
                <Statistic
                  className="link"
                  title={intl.formatMessage({
                    id: 'menu.plugin.system.friendlink',
                  })}
                  value={data.link_count}
                />
              </Col>
              <Col
                flex={1}
                onClick={() => {
                  handleJump('/plugin/interaction/guestbook');
                }}
              >
                <Statistic
                  className="link"
                  title={intl.formatMessage({
                    id: 'menu.plugin.interaction.guestbook',
                  })}
                  value={data.guestbook_count}
                />
              </Col>
              <Col
                flex={1}
                onClick={() => {
                  handleJump('/archive/page');
                }}
              >
                <Statistic
                  className="link"
                  title={intl.formatMessage({ id: 'menu.archive.page' })}
                  value={data.page_count}
                />
              </Col>
              <Col
                flex={1}
                onClick={() => {
                  handleJump('/archive/attachment');
                }}
              >
                <Statistic
                  className="link"
                  title={intl.formatMessage({ id: 'menu.archive.attachment' })}
                  value={data.attachment_count}
                />
              </Col>
              <Col
                flex={1}
                onClick={() => {
                  handleJump('/design/index');
                }}
              >
                <Statistic
                  className="link"
                  title={intl.formatMessage({ id: 'menu.design' })}
                  value={data.template_count}
                />
              </Col>
            </Row>
          </Card>
          <Suspense fallback={null}>
            <Card
              style={{ marginTop: '24px' }}
              bordered={false}
              bodyStyle={{ padding: '0 24px 24px' }}
            >
              <div className="statistic-card">
                <Tabs
                  size="large"
                  defaultActiveKey="traffic"
                  tabBarStyle={{ marginBottom: 24 }}
                >
                  {shopMode && (
                    <TabPane
                      tab={intl.formatMessage({ id: 'menu.statistic.orders' })}
                      key="orders"
                    >
                      <div className="statistic-bar">
                        <Line {...ordersConfig} />
                      </div>
                    </TabPane>
                  )}
                  <TabPane
                    tab={intl.formatMessage({ id: 'menu.statistic.traffic' })}
                    key="traffic"
                  >
                    <div className="statistic-bar">
                      <Line {...trafficConfig} />
                    </div>
                  </TabPane>
                  <TabPane
                    tab={intl.formatMessage({ id: 'menu.statistic.spider' })}
                    key="spider"
                  >
                    <div className="statistic-bar">
                      <Line {...spiderConfig} />
                    </div>
                  </TabPane>
                </Tabs>
              </div>
            </Card>
          </Suspense>
        </Col>
        <Col sm={6} xs={24}>
          <Card title={intl.formatMessage({ id: 'dashboard.login-info' })}>
            <Row gutter={24}>
              <Col span={12}>
                <div className="info-card">
                  <div className="title">
                    <FormattedMessage id="dashboard.login-info.this-time" />
                  </div>
                  <p>
                    {infoData.now_login
                      ? dayjs(infoData.now_login.created_time * 1000).format(
                          'MM-DD HH:mm',
                        )
                      : '-'}
                  </p>
                  <div>{infoData.now_login?.ip}</div>
                </div>
              </Col>
              <Col span={12}>
                <div className="info-card">
                  <div className="title">
                    <FormattedMessage id="dashboard.login-info.last-time" />
                  </div>
                  <p>
                    {infoData.last_login
                      ? dayjs(infoData.last_login.created_time * 1000).format(
                          'MM-DD HH:mm',
                        )
                      : '-'}
                  </p>
                  <div>{infoData.last_login?.ip}</div>
                </div>
              </Col>
            </Row>
          </Card>
          <Card
            style={{ marginTop: '24px' }}
            title={intl.formatMessage({ id: 'dashboard.web-info' })}
          >
            <Row gutter={[16, 24]}>
              <Col span={12}>
                <div className="info-card">
                  <div className="title">
                    <FormattedMessage id="setting.system.site-name" />
                  </div>
                  <div>{infoData.system?.site_name}</div>
                </div>
              </Col>
              <Col span={12}>
                <div className="info-card">
                  <div className="title">
                    <FormattedMessage id="setting.system.template-type" />
                  </div>
                  <div>
                    {infoData.system?.template_type === 2
                      ? intl.formatMessage({
                          id: 'setting.system.template-type.pc-m',
                        })
                      : infoData.system?.template_type === 1
                      ? intl.formatMessage({
                          id: 'setting.system.template-type.code',
                        })
                      : intl.formatMessage({
                          id: 'setting.system.template-type.auto',
                        })}
                  </div>
                </div>
              </Col>
              <Col span={24}>
                <div className="info-card">
                  <div className="title">
                    <FormattedMessage id="setting.system.base-url" />
                  </div>
                  <div>{infoData.system?.base_url}</div>
                </div>
              </Col>
            </Row>
          </Card>
          <Card
            style={{ marginTop: '24px' }}
            title={intl.formatMessage({ id: 'dashboard.soft-info' })}
            extra={
              newVersion &&
              !newVersion.trial && (
                <Tooltip
                  title={intl.formatMessage({
                    id: 'dashboard.soft-info.click-to-upgrade',
                  })}
                >
                  <span
                    onClick={() => handleJump('/tool/upgrade')}
                    className="new-version-tips"
                  >
                    <FormattedMessage id="dashboard.soft-info.new-version" />
                    <span className="version">{newVersion.version}</span>
                  </span>
                </Tooltip>
              )
            }
          >
            <p>
              <FormattedMessage id="dashboard.soft-info.version" />
              <Space>
                {infoData.version}
                {infoData.trial && (
                  <Tag color="orange">
                    <FormattedMessage id="dashboard.soft-info.trial-version" />
                  </Tag>
                )}
              </Space>
            </p>
            <p>
              <FormattedMessage id="dashboard.soft-info.memory-usage" />
              {(infoData.memory_usage / 1024 / 1024).toFixed(1)} MB
            </p>
            <p>
              <FormattedMessage id="dashboard.soft-info.official-web" />
              <a
                href={brand.website}
                target={'_blank'}
                rel="noreferrer"
              >
                {`${brand.website}`}
              </a>
            </p>
            <div>
              <FormattedMessage id="dashboard.soft-info.description" />
            </div>
          </Card>
        </Col>
      </Row>
    </PageContainer>
  );
};

export default Dashboard;
