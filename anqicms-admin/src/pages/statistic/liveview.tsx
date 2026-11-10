import NewContainer from '@/components/NewContainer';
import { getStatisticLiveView } from '@/services';
import { Funnel, Line } from '@ant-design/plots';
import { StatisticCard } from '@ant-design/pro-components';
import { history, useIntl } from '@umijs/max';
import { Button, Col, Row } from 'antd';
import React, { useEffect, useState } from 'react';

const StatisticLiveView: React.FC<any> = () => {
  const [data, setData] = useState<any>({});
  const [newKey, setNewKey] = useState<string>('');
  const [conversion, setConversion] = useState<any[]>([]);
  const intl = useIntl();

  const asyncFetch = async () => {
    getStatisticLiveView()
      .then((res) => {
        setData(res.data);
        const conversionData: any[] = [];
        conversionData.push({
          name: 'views',
          value: res.data.conversion.reduce(
            (total: number, item: any) =>
              item.label === 'views' ? total + (item.value || 0) : total,
            0,
          ),
        });
        conversionData.push({
          name: 'cart',
          value: res.data.conversion.reduce(
            (total: number, item: any) =>
              item.label === 'cart' ? total + (item.value || 0) : total,
            0,
          ),
        });
        conversionData.push({
          name: 'checkout',
          value: res.data.conversion.reduce(
            (total: number, item: any) =>
              item.label === 'checkout' ? total + (item.value || 0) : total,
            0,
          ),
        });
        conversionData.push({
          name: 'order_create',
          value: res.data.conversion.reduce(
            (total: number, item: any) =>
              item.label === 'order_create' ? total + (item.value || 0) : total,
            0,
          ),
        });
        conversionData.push({
          name: 'pay_create',
          value: res.data.conversion.reduce(
            (total: number, item: any) =>
              item.label === 'pay_create' ? total + (item.value || 0) : total,
            0,
          ),
        });
        conversionData.push({
          name: 'payment',
          value: res.data.conversion.reduce(
            (total: number, item: any) =>
              item.label === 'payment' ? total + (item.value || 0) : total,
            0,
          ),
        });

        setConversion(conversionData);
      })
      .catch((error) => {
        console.log('fetch data failed', error);
      });
  };

  const onTabChange = (key: string) => {
    asyncFetch().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    asyncFetch();
  }, []);

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Row gutter={[16, 16]} key={newKey}>
        <Col span={8}>
          <StatisticCard
            title={intl.formatMessage({ id: 'statistic.order.total-amount' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/orders');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.amount || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            title={intl.formatMessage({ id: 'statistic.order.average-amount' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/orders');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.average_amount || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            key={newKey}
            title={intl.formatMessage({ id: 'statistic.order.count' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/orders');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.order_count || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            key={newKey}
            title={intl.formatMessage({ id: 'statistic.product_view' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/products');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.product_view || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            key={newKey}
            title={intl.formatMessage({ id: 'statistic.order.convertion' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/conversion');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Funnel
                {...{
                  data: conversion,
                  xField: 'name',
                  yField: 'value',
                  label: {
                    text: (d: any) => `${d.name}\n${d.value}`,
                  },
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            key={newKey}
            title={intl.formatMessage({ id: 'statistic.customer' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/customers');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.customer || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            key={newKey}
            title={intl.formatMessage({ id: 'statistic.repurchase' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/orders');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.order || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
        <Col span={8}>
          <StatisticCard
            key={newKey}
            title={intl.formatMessage({ id: 'statistic.coupon' })}
            extra={
              <Button
                size="small"
                onClick={() => {
                  history.push('/statistic/coupons');
                }}
              >
                {intl.formatMessage({ id: 'statistic.view-report' })}
              </Button>
            }
            chart={
              <Line
                {...{
                  data: data.coupon || [],
                  xField: 'date',
                  yField: 'value',
                  seriesField: 'label',
                  colorField: 'label',
                  xAxis: {
                    tickCount: 5,
                  },
                  smooth: true,
                }}
              />
            }
          />
        </Col>
      </Row>
    </NewContainer>
  );
};

export default StatisticLiveView;
