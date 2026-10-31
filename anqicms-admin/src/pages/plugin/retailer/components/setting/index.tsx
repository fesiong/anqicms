import { pluginGetRetailerConfig, pluginSaveRetailerConfig } from '@/services/plugin/retailer';
import { ModalForm, ProFormRadio } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

const RetailerSetting: React.FC<any> = (props) => {
  const [setting, setSetting] = useState<any>(null);
  const [visible, setVisible] = useState<boolean>(false);
  const [fetched, setFetched] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetRetailerConfig();
    setSetting(res.data || {});
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (values: any) => {
    values.goods_price = Number(values.goods_price);
    values.share_reward = Number(values.share_reward);
    values.parent_reward = Number(values.parent_reward);
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginSaveRetailerConfig(values)
      .then((res) => {
        message.success(res.msg);
        setVisible(false);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      {fetched && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.retailer.setting' })}
          open={visible}
          initialValues={setting}
          onOpenChange={(flag) => {
            setVisible(flag);
          }}
          onFinish={onSubmit}
        >
          <ProFormRadio.Group
            name="allow_self"
            label={intl.formatMessage({ id: 'plugin.retailer.allow-self' })}
            options={[
              {
                value: 0,
                label: intl.formatMessage({ id: 'plugin.retailer.allow-self.no' }),
              },
              {
                value: 1,
                label: intl.formatMessage({ id: 'plugin.retailer.allow-self.yes' }),
              },
            ]}
            extra={intl.formatMessage({ id: 'plugin.retailer.allow-self.description' })}
          />
          <ProFormRadio.Group
            name="become_retailer"
            label={intl.formatMessage({ id: 'plugin.retailer.become-retailer' })}
            options={[
              {
                value: 0,
                label: intl.formatMessage({ id: 'plugin.retailer.become-retailer.manual' }),
              },
              {
                value: 1,
                label: intl.formatMessage({ id: 'plugin.retailer.become-retailer.auto' }),
              },
            ]}
            extra={intl.formatMessage({ id: 'plugin.retailer.become-retailer.description' })}
          />
        </ModalForm>
      )}
    </>
  );
};

export default RetailerSetting;
