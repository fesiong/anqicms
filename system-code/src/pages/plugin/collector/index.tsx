import { Alert, Button, Card, message, Modal, Space } from 'antd';
import React, { useRef, useState } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import { history, Link } from 'umi';
import CollectorSetting from './components/setting';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import ReplaceKeywords from '@/components/replaceKeywords';
import { PlusOutlined } from '@ant-design/icons';
import { deleteArchive, releaseArchive, getArchives } from '@/services';

const PluginCollector: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [replaceVisible, setReplaceVisible] = useState<boolean>(false);

  const handlePublish = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要发布选中的文档吗？',
      content: '只有文章在草稿箱中，才会被成功发布',
      onOk: async () => {
        const hide = message.loading('正在提交中', 0);
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await releaseArchive({
              id: item,
            });
          }
          hide();
          message.success('发布成功');
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();

          return true;
        } catch (error) {
          hide();
          message.error('发布失败');
          return true;
        }
      },
    });
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的文档吗？',
      onOk: async () => {
        const hide = message.loading('正在删除', 0);
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteArchive({
              id: item,
            });
          }
          hide();
          message.success('删除成功');
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();

          return true;
        } catch (error) {
          hide();
          message.error('删除失败');
          return true;
        }
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: '编号',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: '标题',
      dataIndex: 'title',
      hideInSearch: true,
      render: (dom, entity) => {
        return (
          <div style={{maxWidth: 400}}><a href={entity.link} target="_blank">
          {dom}
        </a></div>
        );
      },
    },
    {
      title: 'thumb',
      dataIndex: 'thumb',
      hideInSearch: true,
      render: (text, record) => {
        return (
          text ? <img src={record.thumb} className='list-thumb' /> : null
        );
      },
    },
    {
      title: '内容模型',
      dataIndex: 'module_name',
      hideInSearch: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: '草稿',
          status: 'Default',
        },
        1: {
          text: '正常',
          status: 'Success',
        },
        2: {
          text: '待发布',
          status: 'Default',
        },
      }
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
        {record.status == 0 && <a
          className="text-red"
          key="recover"
          onClick={async () => {
            await handlePublish([record.id]);
          }}
        >
          发布
        </a>}
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.id]);
            }}
          >
            删除
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card>
      <Alert
            message={
              <div>
                采集文章需要先设置核心关键词，请检查“关键词库管理”功能，并添加相应的关键词。更多采集和伪原创设置，请点击{' '}

              </div>
            }
          />

<ProTable<any>
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <CollectorSetting onCancel={() => {}} key="setting">
                  <Button>
            采集和伪原创设置
          </Button>
                </CollectorSetting>
          ,
          <Button
            key="keywords"
            onClick={() => {
              history.push('/plugin/keyword')
            }}
          >
            关键词库管理
          </Button>,
          <Button
            key="replace"
            onClick={() => {
              setReplaceVisible(true);
            }}
          >
            批量替换关键词
          </Button>,
          <Button
            type="primary"
            key="add"
            onClick={() => {
              history.push('/archive/detail');
            }}
          >
            <PlusOutlined /> 手动采集
          </Button>,
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
          <Button
            size={'small'}
            onClick={async () => {
              await handlePublish(selectedRowKeys);
            }}
          >
            批量发布
          </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRemove(selectedRowKeys);
              }}
            >
              批量删除
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              取消选择
            </Button>
          </Space>
        )}
        request={(params, sort) => {
          params.collect = true;
          return getArchives(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      </Card>
      <ReplaceKeywords
        visible={replaceVisible}
        onCancel={() => {
          setReplaceVisible(false);
        }}
      />
    </PageContainer>
  );
};

export default PluginCollector;
