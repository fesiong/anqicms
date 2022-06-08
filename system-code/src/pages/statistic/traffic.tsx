import React, { useEffect, useState } from 'react';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { StatisticCard } from '@ant-design/pro-card';
import { DualAxes } from '@ant-design/plots';
import { getStatisticTraffic } from '@/services/statistic';

const StatisticTraffic: React.FC<any> = (props) => {
  const [data, setData] = useState<any[]>([]);

  useEffect(() => {
    asyncFetch();
  }, []);

  const asyncFetch = () => {
    getStatisticTraffic()
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
  const config = {
    data: [data, data],
    //padding: 'auto',
    xField: 'statistic_date',
    yField: ['total', 'ips'],
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  return (
    <PageHeaderWrapper>
      <StatisticCard
        title="蜘蛛统计"
        tip="这里统计的是非蜘蛛访问的IP以及访问次数，具体可以查看详情记录"
        chart={<DualAxes {...config} />}
      />
    </PageHeaderWrapper>
  );
};

export default StatisticTraffic;
