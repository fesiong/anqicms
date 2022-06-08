import React, { useEffect, useState } from 'react';
import ProForm, { ProFormTextArea, ProFormRadio, ProFormText } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Card, Col, message, Radio, Row, Space, Tag } from 'antd';
import { pluginGetPush, pluginSavePush } from '@/services/plugin/push';

const PluginPush: React.FC<any> = (props) => {
  const [pushSetting, setPushSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetPush();
    let setting = res.data || {};
    setPushSetting(setting);
    setFetched(true);
  };

  const onSubmit = async (values: any) => {
    values = Object.assign(pushSetting, values)
    pluginSavePush(values)
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
        <Alert message="搜索引擎推送功能支持百度搜索、必应搜索的主动推送，其他搜索引擎虽然没有主动推送功能，但部分搜索引擎依然可以使用JS推送。" />
        <div className="mt-normal">
          {fetched && (
            <ProForm onFinish={onSubmit} initialValues={pushSetting}>
              <Card size="small" title="百度搜索主动推送" bordered={false}>
                <ProFormText
                  name="baidu_api"
                  label="推送接口地址"
                  extra="如：http://data.zz.baidu.com/urls?site=https://www.kandaoni.com&token=DTHpH8Xn99BrJLBY"
                />
              </Card>
              <Card size="small" title="必应搜索主动推送" bordered={false}>
                <ProFormText
                  name="bing_api"
                  label="推送接口地址"
                  extra="如：https://ssl.bing.com/webmaster/api.svc/json/SubmitUrlbatch?apikey=sampleapikeyEDECC1EA4AE341CC8B6（注意该APIkey在必应工具右上角的设置中设置）"
                />
              </Card>
              <Card size="small" title="360/头条等JS自动提交" bordered={false}>
                <ProFormTextArea
                  name="js_code"
                  fieldProps={{
                    rows: 10,
                  }}
                  label="推送接口地址"
                  extra="可以放置百度JS自动提交、360自动收录、头条自动收录等JS代码。"
                />
              </Card>
            </ProForm>
          )}
        </div>
      </Card>
    </PageHeaderWrapper>
  );
};

export default PluginPush;
