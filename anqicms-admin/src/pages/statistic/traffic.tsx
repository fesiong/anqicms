import NewContainer from '@/components/NewContainer';
import { getStatisticTraffic } from '@/services/statistic';
import { Line } from '@ant-design/plots';
import { StatisticCard } from '@ant-design/pro-components';
import { history, useIntl } from '@umijs/max';
import { Button } from 'antd';
import React, { useEffect, useState } from 'react';

const StatisticTraffic: React.FC<any> = () => {
  const [data, setData] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const asyncFetch = async () => {
    getStatisticTraffic()
      .then((res) => {
        setData(res.data);
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

  const config = {
    data: data,
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
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <StatisticCard
        key={newKey}
        title={intl.formatMessage({ id: 'statistic.traffic' })}
        tooltip={intl.formatMessage({ id: 'statistic.traffic.tips' })}
        extra={
          <Button
            size="small"
            onClick={() => {
              history.push('/statistic/detail?type=traffic');
            }}
          >
            {intl.formatMessage({ id: 'menu.statistic.detail' })}
          </Button>
        }
        chart={<Line {...config} />}
      />
    </NewContainer>
  );
};

export default StatisticTraffic;
