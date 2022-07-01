import { Alert, Button, Card, Space } from 'antd';
import React from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import { Link } from 'umi';
import CollectorSetting from './components/setting';

const PluginCollector: React.FC = () => {
  return (
    <PageContainer>
      <Card>
        <Space direction="vertical" size={20}>
          <Alert
            message={
              <div>
                采集文章需要先设置核心关键词，请检查“关键词库管理”功能，并添加相应的关键词。更多采集和伪原创设置，请点击{' '}
                <CollectorSetting onCancel={() => {}}>
                  <a>采集和伪原创设置</a>
                </CollectorSetting>
              </div>
            }
          />
          <Alert
            message={
              <div>
                <p>
                  已采集的文章，请到“<Link to="/archive/list">文章管理</Link>”中查看
                </p>
                <div>
                  关键词管理，请到“<Link to="/plugin/keyword">关键词库管理</Link>”中查看
                </div>
              </div>
            }
          />
        </Space>
      </Card>
    </PageContainer>
  );
};

export default PluginCollector;
