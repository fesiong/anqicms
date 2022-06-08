import React, { useEffect, useState } from 'react';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { StatisticCard } from '@ant-design/pro-card';
import { Line } from '@ant-design/plots';
import { getStatisticSpider } from '@/services/statistic';

const StatisticSpider: React.FC<any> = (props) => {
  const [data, setData] = useState<any[]>([]);

  useEffect(() => {
    asyncFetch();
  }, []);

  const asyncFetch = () => {
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
  const config = {
    data,
    //padding: 'auto',
    xField: 'statistic_date',
    yField: 'total',
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
        tip="这里统计的是蜘蛛爬取的次数，具体可以查看详情记录"
        chart={<Line {...config} />}
      />
    </PageHeaderWrapper>
  );
};

export default StatisticSpider;
