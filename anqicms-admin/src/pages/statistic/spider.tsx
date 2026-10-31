import NewContainer from '@/components/NewContainer';
import { getStatisticSpider } from '@/services/statistic';
import { Line } from '@ant-design/plots';
import { StatisticCard } from '@ant-design/pro-components';
import { history, useIntl } from '@umijs/max';
import { Button } from 'antd';
import React, { useEffect, useState } from 'react';

const StatisticSpider: React.FC<any> = () => {
  const [data, setData] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const asyncFetch = async () => {
    getStatisticSpider()
      .then((res) => {
        setData(res.data);
      })
      .catch((error) => {
        console.log('fetch data failed', error);
      });
    // fetch('https://gw.alipayobjects.com/os/bmw-prod/1d565782-dde4-4bb6-8946-ea6a38ccf184.json')
    //   .then((response) => response.json())
    //   .then((json) => setData(json))
    //   .catch((error) => {
    //     console.log('fetch data failed', error);
    //   });
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
    data,
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
        title={intl.formatMessage({ id: 'statistic.spider' })}
        tooltip={intl.formatMessage({ id: 'statistic.spider.tips' })}
        extra={
          <Button
            size="small"
            onClick={() => {
              history.push('/statistic/detail?type=spider');
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

export default StatisticSpider;
