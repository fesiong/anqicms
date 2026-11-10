import NewContainer from '@/components/NewContainer';
import { getSettingCurrency, saveSettingCurrency } from '@/services';
import { ProForm, ProFormText } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card, message } from 'antd';
import React, { useEffect, useState } from 'react';

const SettingCurrencyFrom: React.FC<any> = () => {
  const [setting, setSetting] = useState<any>(null);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingCurrency();
    let setting = res.data || null;
    setSetting(setting);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingCurrency(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        {setting && (
          <ProForm
            initialValues={setting}
            onFinish={onSubmit}
            title={intl.formatMessage({ id: 'menu.setting.currency' })}
          >
            <ProFormText
              name="name"
              label={intl.formatMessage({
                id: 'setting.currency.currency-name',
              })}
              width="lg"
            />
            <ProFormText
              name="currency"
              label={intl.formatMessage({
                id: 'setting.currency.currency-code',
              })}
              width="lg"
              extra={intl.formatMessage({
                id: 'setting.currency.currency-code.description',
              })}
            />
            <ProFormText
              name="currency"
              label={intl.formatMessage({
                id: 'setting.currency.currency-symbol',
              })}
              width="lg"
              extra={intl.formatMessage({
                id: 'setting.currency.currency-symbol.description',
              })}
            />
          </ProForm>
        )}
      </Card>
    </NewContainer>
  );
};

export default SettingCurrencyFrom;
