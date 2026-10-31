import NewContainer from '@/components/NewContainer';
import { getSettingSafe, saveSettingSafe } from '@/services';
import {
  ProForm,
  ProFormInstance,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const SettingSafeFrom: React.FC<any> = () => {
  const formRef = useRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>(null);
  const [activeTabKey, setActiveTabKey] = useState<string>('base');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingSafe();
    let setting = res.data || null;
    setSetting(setting);
    formRef.current?.setFieldsValue(setting);
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
    // 页面按 tab 条件渲染，onFinish 的 values 只含当前 tab 字段，
    // 后端整包覆盖保存会清空其它 tab 的配置，必须取全量表单值
    const allValues: any = {
      ...(formRef.current?.getFieldsValue(true) || values),
    };
    allValues.captcha = Number(allValues.captcha);
    allValues.admin_captcha_off = Number(allValues.admin_captcha_off);
    allValues.daily_limit = Number(allValues.daily_limit);
    allValues.content_limit = Number(allValues.content_limit);
    allValues.interval_limit = Number(allValues.interval_limit);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingSafe(allValues)
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

  const tabContnet: Record<string, React.ReactNode> = {
    base: (
      <div>
        <ProFormRadio.Group
          name="admin_captcha_off"
          label={intl.formatMessage({ id: 'setting.safe.admin-captcha' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.safe.admin-captcha.description',
          })}
        />

        <ProFormTextArea
          name="ua_forbidden"
          label={intl.formatMessage({ id: 'setting.safe.ua-forbidden' })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.safe.ua-forbidden.description',
          })}
        />
        <ProFormTextArea
          name="ip_forbidden"
          label={intl.formatMessage({ id: 'setting.safe.ip-forbidden' })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.safe.ip-forbidden.description',
          })}
        />
        <ProFormRadio.Group
          name="api_open"
          label={intl.formatMessage({ id: 'setting.safe.api-open' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.safe.api-open.description',
          })}
        />
        <ProFormRadio.Group
          name="api_publish"
          label={intl.formatMessage({ id: 'setting.safe.api-publish' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.safe.api-publish.draft',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.safe.api-publish.normal',
              }),
            },
          ]}
        />
      </div>
    ),
    verify: (
      <div>
        <ProFormRadio.Group
          name="captcha"
          label={intl.formatMessage({ id: 'setting.safe.captcha' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.safe.captcha.description',
          })}
        />
        <ProFormText
          name="daily_limit"
          label={intl.formatMessage({ id: 'setting.safe.daily-limit' })}
          width="lg"
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'setting.safe.daily-limit.suffix',
            }),
          }}
          extra={intl.formatMessage({
            id: 'setting.safe.daily-limit.description',
          })}
        />
        <ProFormText
          name="content_limit"
          label={intl.formatMessage({ id: 'setting.safe.content-limit' })}
          width="lg"
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'setting.safe.content-limit.suffix',
            }),
          }}
          extra={intl.formatMessage({
            id: 'setting.safe.daily-limit.description',
          })}
        />
        <ProFormText
          name="interval_limit"
          label={intl.formatMessage({ id: 'setting.safe.interval-limit' })}
          width="lg"
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'setting.safe.interval-limit.suffix',
            }),
          }}
          extra={intl.formatMessage({
            id: 'setting.safe.daily-limit.description',
          })}
        />
        <ProFormTextArea
          name="content_forbidden"
          label={intl.formatMessage({
            id: 'setting.safe.content-forbidden',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.safe.content-forbidden.description',
          })}
        />
      </div>
    ),
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProForm
        formRef={formRef}
        initialValues={setting}
        onFinish={onSubmit}
        title={intl.formatMessage({ id: 'menu.setting.safe' })}
      >
        <Card
          className="mb-normal"
          key={newKey}
          activeTabKey={activeTabKey}
          onTabChange={(tabKey) => setActiveTabKey(tabKey)}
          tabList={[
            {
              key: 'base',
              label: intl.formatMessage({ id: 'setting.tab.security-base' }),
            },
            {
              key: 'verify',
              label: intl.formatMessage({ id: 'setting.tab.security-comment' }),
            },
          ]}
        >
          {tabContnet[activeTabKey] || null}
        </Card>
      </ProForm>
    </NewContainer>
  );
};

export default SettingSafeFrom;
