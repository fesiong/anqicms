import React, { useState, useEffect } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import { Card } from 'antd';
import './index.less';

const DesignMarket: React.FC = () => {
  const [height, setHeight] = useState(0);

  useEffect(() => {
    getHeight();
    window.addEventListener('resize', getHeight);
    return () => {
        // 组件销毁时移除监听事件
        window.removeEventListener('resize', getHeight);
    }
  }, []);

  const getHeight = () => {
    let num = window?.innerHeight - 260;
    if (num < 450) {
      num = 450;
    } else if (num > 900) {
      num = 900;
    }

    setHeight(num);
  }

  return (
    <PageContainer>
      <Card>
        <iframe className='frame-page' src='https://www.kandaoni.com/category/27' height={height}></iframe>
      </Card>
    </PageContainer>
  );
};

export default DesignMarket;
