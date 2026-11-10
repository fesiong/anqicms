import NewContainer from '@/components/NewContainer';
import AttachmentSelect from '@/components/attachment';
import {
  deleteArchive,
  pluginGetOrderReviews,
  pluginSaveOrderReview,
} from '@/services';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Image, Modal, Space, Tag, Tooltip, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import './index.less';

let updating = false;

let lastParams: any = {
  module_id: 0,
  category_id: 0,
  status: 'ok',
  flag: '',
  exact: false,
};

const ProductReview: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentItem, setCurrentItem] = useState<any>({});
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  useEffect(() => {}, []);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteArchive({
              id: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();

          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditReview = async (record: any) => {
    setCurrentItem(record);
    setEditVisible(true);
  };

  const getExactCount = () => {
    message.loading(intl.formatMessage({ id: 'content.count.loading' }));
    lastParams.exact = true;
    actionRef.current?.reload?.();
  };

  const onSubmit = async (values: any) => {
    if (updating) return;
    updating = true;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    try {
      const data = Object.assign({}, currentItem, values);
      await pluginSaveOrderReview(data);
      hide();
      message.success(intl.formatMessage({ id: 'content.submit.success' }));
      setEditVisible(false);
      setCurrentItem({});
      actionRef.current?.reloadAndRest?.();
      updating = false;
      return true;
    } catch (error) {
      hide();
      message.error(intl.formatMessage({ id: 'content.submit.failure' }));
      updating = false;
      return false;
    }
  };

  const handleSelectImages = (rows: any) => {
    for (const row of rows) {
      let exists = false;
      for (let i in currentItem.images) {
        if (currentItem.images[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        currentItem.images.push(row.file_path);
      }
    }
    setCurrentItem((prev: any) => ({
      ...prev,
      images: {
        ...currentItem.images,
      },
    }));
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleCleanImages = (index: number, e: any) => {
    e.stopPropagation();
    currentItem.images.splice(index, 1);
    setCurrentItem((prev: any) => ({
      ...prev,
      images: {
        ...currentItem.images,
      },
    }));
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.product.name' }),
      dataIndex: 'archive_title',
      render: (dom, entity) => {
        return (
          <div style={{ maxWidth: 400 }}>
            {dom}
            {entity.sku_title && <div>{entity.sku_title}</div>}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.user.name' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.content.name' }),
      dataIndex: 'content',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.score' }),
      dataIndex: 'score',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.vote-count' }),
      dataIndex: 'vote_count',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.review.status.hide' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.review.status.show' }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.review.created-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'created_time',
      render: (_, record) => {
        return (
          <div>
            {dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm')}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditReview(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            onClick={async () => {
              await handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={onTabChange}>
      <ProTable<any>
        key={newKey}
        actionRef={actionRef}
        rowKey="id"
        search={{
          span: { xs: 24, sm: 12, md: 8, lg: 8, xl: 8, xxl: 8 },
          defaultCollapsed: false,
        }}
        formRef={formRef}
        form={{
          initialValues: lastParams,
        }}
        toolBarRender={() => []}
        columnsState={{
          persistenceKey: 'archive-table',
          persistenceType: 'localStorage',
        }}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space wrap>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRemove(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-delete" />
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={async (params, sort) => {
          // eslint-disable-next-line guard-for-in
          for (let i in sort) {
            params.sort = i;
            params.order = sort[i] === 'ascend' ? 'asc' : 'desc';
          }
          params.exact = lastParams.exact;
          lastParams = params;
          const res = await pluginGetOrderReviews(params);
          lastParams.exact = res.exact || false;
          return res;
        }}
        columns={columns}
        rowSelection={{
          preserveSelectedRowKeys: true,
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          showQuickJumper: true,
          showSizeChanger: true,
          defaultCurrent: lastParams.current,
          defaultPageSize: lastParams.pageSize,
          showTotal: (total, range) => (
            <div>
              {lastParams.exact === false && (
                <Tooltip
                  title={intl.formatMessage({
                    id: 'content.count.tips.tooltip',
                  })}
                >
                  <Tag className="link" color="orange" onClick={getExactCount}>
                    <FormattedMessage id="content.count.tips" />
                  </Tag>
                </Tooltip>
              )}
              <FormattedMessage
                id="content.archive.total"
                values={{ total: total, range0: range[0], range1: range[1] }}
              />
            </div>
          ),
        }}
      />
      {editVisible && (
        <ModalForm
          open={editVisible}
          onOpenChange={setEditVisible}
          initialValues={currentItem}
          onFinish={onSubmit}
        >
          <ProFormTextArea
            name="content"
            label={intl.formatMessage({ id: 'plugin.review.content.name' })}
          />
          <ProFormDigit
            name="score"
            label={intl.formatMessage({ id: 'plugin.review.score' })}
            min={1}
            max={5}
          />
          <ProFormRadio.Group
            name="status"
            label={intl.formatMessage({ id: 'plugin.review.status' })}
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.review.status.hide' }),
                value: 0,
              },
              {
                label: intl.formatMessage({ id: 'plugin.review.status.show' }),
                value: 1,
              },
            ]}
          />
          <Card
            className="aside-card"
            size="small"
            title={intl.formatMessage({ id: 'plugin.review.images' })}
          >
            {currentItem?.images?.length
              ? currentItem.images.map((item: string, index: number) => (
                  <div className="ant-upload-item" key={index}>
                    <Image
                      preview={{
                        src: item,
                      }}
                      src={item}
                    />
                    <span
                      className="delete"
                      onClick={handleCleanImages.bind(this, index)}
                    >
                      <DeleteOutlined />
                    </span>
                  </div>
                ))
              : null}
            <AttachmentSelect
              onSelect={handleSelectImages}
              open={false}
              multiple={true}
            >
              <div className="ant-upload-item">
                <div className="add">
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>
                    <FormattedMessage id="setting.system.upload" />
                  </div>
                </div>
              </div>
            </AttachmentSelect>
          </Card>
        </ModalForm>
      )}
    </NewContainer>
  );
};

export default ProductReview;
