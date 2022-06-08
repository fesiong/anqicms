import React, { useEffect, useState } from 'react';
import ProForm, { ProFormText } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Card, message } from 'antd';
import { getSettingIndex, saveSettingIndex } from '@/services/setting';

const SettingIndexFrom: React.FC<any> = (props) => {
  const [setting, setSetting] = useState<any>(null);
  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await getSettingIndex();
    let setting = res.data || null;
    setSetting(setting);
  };

  const onSubmit = async (values: any) => {
    saveSettingIndex(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      });
  };

  return (
    <PageHeaderWrapper>
      <Card>
        {setting && (
          <ProForm initialValues={setting} onFinish={onSubmit} title="首页TDK设置">
            <ProFormText name="seo_title" label="首页标题" width="lg" />
            <ProFormText
              name="seo_keywords"
              label="首页关键词"
              width="lg"
              extra={
                <div>
                  多个关键词请用<cite>,</cite>隔开
                </div>
              }
            />
            <ProFormText name="seo_description" label="首页描述" width="lg" />
          </ProForm>
        )}
      </Card>
    </PageHeaderWrapper>
  );
};

export default SettingIndexFrom;
