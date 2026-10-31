import NewContainer from '@/components/NewContainer';
import { getSettingIndex, saveSettingIndex } from '@/services/setting';
import { QuestionCircleOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormInstance,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Card, message, Popover } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const SettingIndexFrom: React.FC<any> = () => {
  const formRef = useRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>(null);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingIndex();
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
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingIndex(values)
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
        <ProForm
          formRef={formRef}
          initialValues={setting}
          onFinish={onSubmit}
          title={intl.formatMessage({ id: 'menu.setting.tdk' })}
        >
          <ProFormText
            name="seo_title"
            label={intl.formatMessage({ id: 'setting.index.title' })}
            width="lg"
            extra={
              <div>
                <FormattedMessage id="setting.index.title.tips" />
                <Popover
                  content={
                    <div style={{ whiteSpace: 'pre-wrap' }}>
                      {intl.formatMessage({
                        id: 'setting.index.params.tips',
                      })}
                    </div>
                  }
                  title={intl.formatMessage({
                    id: 'setting.index.title.tips',
                  })}
                >
                  {' '}
                  <QuestionCircleOutlined />
                </Popover>
              </div>
            }
          />
          <ProFormText
            name="seo_keywords"
            label={intl.formatMessage({ id: 'setting.index.keywords' })}
            width="lg"
            extra={
              <div>
                <FormattedMessage id="setting.index.keywords.tips" />
              </div>
            }
          />
          <ProFormTextArea
            name="seo_description"
            label={intl.formatMessage({ id: 'setting.index.description' })}
            width="lg"
          />
          <ProFormText
            name="sep"
            label={intl.formatMessage({ id: 'setting.index.sep' })}
            width="lg"
            extra={
              <div>
                <FormattedMessage id="setting.index.sep.tips" />
              </div>
            }
          />
        </ProForm>
      </Card>
    </NewContainer>
  );
};

export default SettingIndexFrom;
