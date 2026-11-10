import NewContainer from '@/components/NewContainer';
import { getStatisticCoupon } from '@/services';
import { Line } from '@ant-design/plots';
import { StatisticCard } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import React, { useEffect, useState } from 'react';

const StatisticCoupons: React.FC<any> = () => {
  const [data, setData] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const asyncFetch = async () => {
    getStatisticCoupon()
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
        title={intl.formatMessage({ id: 'menu.statistic.coupons' })}
        chart={<Line {...config} />}
      />
    </NewContainer>
  );
};

export default StatisticCoupons;
