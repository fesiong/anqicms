import NewContainer from '@/components/NewContainer';
import {
  pluginGetGoogleMerchantSetting,
  pluginGoogleMerchantAuthCallback,
} from '@/services';
import { LoadingOutlined } from '@ant-design/icons';
import { history, useIntl } from '@umijs/max';
import { Card, Space, message } from 'antd';
import React, { useEffect, useState } from 'react';
import '../index.less';

const GoogleMerchantCallback: React.FC<any> = () => {
  const [googleSetting, setGoogleSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetGoogleMerchantSetting();
    let setting = res.data || {};

    setGoogleSetting(setting);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.get('code')) {
      const code = urlParams.get('code');
      const state = urlParams.get('state');
      const body = {
        code,
        state,
      };
      pluginGoogleMerchantAuthCallback(body).then((res) => {
        if (res.code === 0) {
          message.success(
            intl.formatMessage({
              id: 'plugin.googlemerchant.google-account.connected',
            }),
          );
        } else {
          message.error(res.msg);
        }

        history.replace({
          pathname: '/promotion/googlemerchant',
        });
      });
    } else {
      const err = urlParams.get('error') || 'error';
      message.error(intl.formatMessage({ id: err }));
      history.replace({
        pathname: '/promotion/googlemerchant',
      });
    }
  }, []);

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        title={intl.formatMessage({ id: 'menu.promotion.googlemerchant' })}
      >
        <Space align="center">
          <LoadingOutlined />
        </Space>
      </Card>
    </NewContainer>
  );
};

export default GoogleMerchantCallback;
