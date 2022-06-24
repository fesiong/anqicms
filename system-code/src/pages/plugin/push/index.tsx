import React, { useEffect, useRef, useState } from 'react';
import ProForm, { ProFormTextArea, ProFormRadio, ProFormText, ModalForm } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Button, Card, Col, message, Modal, Radio, Row, Space, Tag } from 'antd';
import { pluginGetPush, pluginGetPushLogs, pluginSavePush } from '@/services/plugin/push';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import moment from 'moment';

const PluginPush: React.FC<any> = (props) => {
  const actionRef = useRef<ActionType>();
  const [pushSetting, setPushSetting] = useState<any>({});
  const [jsCodes, setJsCodes] = useState<any[]>([]);
  const [fetched, setFetched] = useState<boolean>(false);
  const [logVisible, setLogVisible] = useState<boolean>(false);
  const [editCodeVisible, setEditCodeVisible] = useState<boolean>(false);
  const [currentIndex, setCurrentIndex] = useState<number>(-1);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetPush();
    let setting = res.data || {};
    setPushSetting(setting);
    setJsCodes(setting.js_codes || [])
    setFetched(true);
  };

  const onSubmit = async (values: any) => {
    values = Object.assign(pushSetting, values)
    pushSetting.js_codes = jsCodes
    pluginSavePush(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      });
  };

  const handleShowAddJs = () => {
    let index = jsCodes.push({name: '', value: ''})-1;
    setCurrentIndex(index)
    setJsCodes([].concat(...jsCodes))
    setEditCodeVisible(true);
  }

  const handleEditJs = (row: any, index: number) => {
    setCurrentIndex(index)
   setEditCodeVisible(true);
  }

  const handleSaveEditJs = async (values: any) => {
    jsCodes[currentIndex] = values;
    setJsCodes([].concat(...jsCodes))
    setEditCodeVisible(false);
    actionRef.current?.reloadAndRest?.();
  }

  const handleRemoveJs = (index: number) => {
    jsCodes.splice(index, 1);
    setJsCodes([].concat(...jsCodes))
  }

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

  const jsColumns: ProColumns<any>[] = [
    {
      title: '名称',
      width: 200,
      dataIndex: 'name',
    },
    {
      title: '代码',
      dataIndex: 'value',
      render: (text, record) => <div style={{maxHeight: 60, overflow: 'hidden', textOverflow: 'ellipsis'}}>
        {text}
      </div>,
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      width: 150,
      render: (_, record, index) => (
        <Space size={20}>
          <a
            key="check"
            onClick={() => {
              handleEditJs(record, index);
            }}
          >
            编辑
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemoveJs(index);
              actionRef.current?.reloadAndRest?.();
            }}
          >
            删除
          </a>
        </Space>
      ),
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
              <ProTable<any>
        actionRef={actionRef}
        rowKey="name"
        search={false}
        toolBarRender={() => [
          <Button
            key="add"
            onClick={() => {
              handleShowAddJs();
            }}
          >
            添加JS代码
          </Button>,
        ]}
        tableAlertOptionRender={false}
        request={async (params, sort) => {
          console.log(jsCodes)
          return {
            data: jsCodes
          }
        }
        }
        columns={jsColumns}
      />
                  <div>
                    <p>可以放置百度JS自动提交、360自动收录、头条自动收录等JS代码。</p>
                    <p>这些代码需要在模板中手动调用，请在公共的模板结尾添加 <Tag>{'{{- pluginJsCode|safe }}'}</Tag> 代码来调用。</p>
                  </div>
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
      {editCodeVisible && <ModalForm visible={editCodeVisible} onFinish={handleSaveEditJs} onVisibleChange={(flag) => {
        setEditCodeVisible(flag)
      }} initialValues={jsCodes[currentIndex]}>
          <ProFormText
                  name="name"
                  label="代码名称"
                  placeholder="如：百度统计"
                />
              <ProFormTextArea
                  name="value"
                  label="JS代码"
                  extra="需要包含<script>开头，和</script>结尾"
                  fieldProps={{
                    rows: 8
                  }}
                />
      </ModalForm>}
    </PageHeaderWrapper>
  );
};

export default PluginPush;
