import NewContainer from '@/components/NewContainer';
import { getStatisticSearch } from '@/services';
import { Line } from '@ant-design/plots';
import { StatisticCard } from '@ant-design/pro-components';
import { history, useIntl } from '@umijs/max';
import { Button } from 'antd';
import React, { useEffect, useState } from 'react';

const StatisticSearch: React.FC<any> = () => {
  const [data, setData] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const asyncFetch = async () => {
    getStatisticSearch()
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
        title={intl.formatMessage({ id: 'menu.statistic.search' })}
        extra={
          <Button
            size="small"
            onClick={() => {
              history.push('/statistic/search/detail');
            }}
          >
            {intl.formatMessage({ id: 'menu.statistic.searchdetail' })}
          </Button>
        }
        chart={<Line {...config} />}
      />
    </NewContainer>
  );
};

export default StatisticSearch;
