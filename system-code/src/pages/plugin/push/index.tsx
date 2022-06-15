import React, { useEffect, useState } from 'react';
import ProForm, { ProFormTextArea, ProFormRadio, ProFormText } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Button, Card, Col, message, Modal, Radio, Row, Space, Tag } from 'antd';
import { pluginGetPush, pluginGetPushLogs, pluginSavePush } from '@/services/plugin/push';
import ProTable, { ProColumns } from '@ant-design/pro-table';
import moment from 'moment';

const PluginPush: React.FC<any> = (props) => {
  const [pushSetting, setPushSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [logVisible, setLogVisible] = useState<boolean>(false);

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

  const handleShowPushLog = () => {
    setLogVisible(true)
  }

  const columns: ProColumns<any>[] = [
    {
      title: '时间',
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '搜索引擎',
      width: 160,
      dataIndex: 'spider',
    },
    {
      title: '推送结果',
      dataIndex: 'result',
    },
  ];

  return (
    <PageHeaderWrapper>
      <Card>
        <Alert message={<div>
          <span>搜索引擎推送功能支持百度搜索、必应搜索的主动推送，其他搜索引擎虽然没有主动推送功能，但部分搜索引擎依然可以使用JS推送。</span>
          <Button size='small' onClick={handleShowPushLog}>查看最近推送记录</Button>
          </div>} />
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
      <Modal title='查看最近推送记录' width={900} visible={logVisible} onCancel={() => {
        setLogVisible(false)
      }} onOk={() => {
        setLogVisible(false)
      }}>
        <ProTable<any>
        rowKey="id"
        search={false}
        pagination={false}
        toolBarRender={false}
        request={(params, sort) => {
          return pluginGetPushLogs(params);
        }}
        columns={columns}
      />
      </Modal>
    </PageHeaderWrapper>
  );
};

export default PluginPush;
