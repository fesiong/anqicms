import React, { useEffect, useState } from 'react';
import ProForm, { ProFormTextArea, ProFormRadio, ProFormText } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Button, Card, Col, message, Radio, Row, Space, Tag } from 'antd';
import { pluginGetRobots, pluginSaveRobots } from '@/services/plugin/robots';
import { useModel } from 'umi';

const PluginRobots: React.FC<any> = (props) => {
  const { initialState } = useModel('@@initialState');
  const [pushSetting, setPushSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetRobots();
    let setting = res.data || {};
    setPushSetting(setting);
    setFetched(true);
  };

  const onSubmit = async (values: any) => {
    pluginSaveRobots(values)
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
        <Alert message="Robots是网站告诉搜索引擎蜘蛛哪些页面可以抓取，哪些页面不能抓取的配置" />
        <div className="mt-normal">
          {fetched && (
            <ProForm onFinish={onSubmit} initialValues={pushSetting}>
              <ProFormTextArea
                name="robots"
                fieldProps={{
                  rows: 15,
                }}
                label="Robots内容"
                extra={
                  <div>
                    <p>
                      1、robots.txt可以告诉百度您网站的哪些页面可以被抓取，哪些页面不可以被抓取。
                    </p>
                    <p>2、您可以通过Robots工具来创建、校验、更新您的robots.txt文件。</p>
                  </div>
                }
              />
            </ProForm>
          )}
        </div>
              <div className='mt-normal'>
        <Button
                    onClick={() => {
                      window.open(initialState.system?.base_url+'/robots.txt')
                    }}
                  >
                    查看Robots
                  </Button>
            </div>
      </Card>
    </PageHeaderWrapper>
  );
};

export default PluginRobots;
