import NewContainer from '@/components/NewContainer';
import { anqiDownloadTemplate } from '@/services';
import { history, useIntl, useModel } from '@umijs/max';
import { Card, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import './index.less';

const DesignMarket: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const actionRef = useRef(null);
  const [height, setHeight] = useState(0);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const receiveDownload = (e: any) => {
    const data = e.data || {};
    if (data.action === 'download') {
      const hide = message.loading(
        intl.formatMessage({ id: 'design.market.downloading' }),
        0,
      );
      anqiDownloadTemplate({
        template_id: Number(data.id),
      })
        .then((res) => {
          message.info(res.msg);
          if (res.code === 0) {
            setTimeout(() => {
              history.push('/design/index');
            }, 1000);
          }
        })
        .finally(() => {
          hide();
        });
    }
  };

  const getHeight = () => {
    let num = window?.innerHeight - 260;
    if (num < 450) {
      num = 450;
    } else if (num > 900) {
      num = 900;
    }

    setHeight(num);
  };

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  useEffect(() => {
    getHeight();
    window.addEventListener('resize', getHeight);
    window.addEventListener('message', receiveDownload);
    return () => {
      // 组件销毁时移除监听事件
      window.removeEventListener('resize', getHeight);
      window.removeEventListener('message', receiveDownload);
    };
  }, []);

  const handleIframe = () => {
    if (initialState && initialState.anqiUser) {
      const anqiUser = initialState.anqiUser;
      const token = anqiUser.token;
      actionRef.current?.contentWindow?.postMessage(
        {
          token,
        },
        '*',
      );
    }
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <iframe
          ref={actionRef}
          className="frame-page"
          src="https://www.anqicms.com/design"
          height={height}
          onLoad={handleIframe}
        ></iframe>
      </Card>
    </NewContainer>
  );
};

export default DesignMarket;
