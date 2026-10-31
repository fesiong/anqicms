import AttachmentSelect from '@/components/attachment';
import {
  pluginGetCommunicationMessages,
  pluginGetCommunicationOrderInfo,
  pluginSaveCommunicationOrderImages,
  pluginSendCommunicationMessage,
} from '@/services';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { ModalForm, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Avatar,
  Button,
  Divider,
  Image,
  Input,
  List,
  Pagination,
  Space,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import './index.less';

export type CommunicationOrderDetailProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  order: any;
};

const CommunicationOrderDetail: React.FC<CommunicationOrderDetailProps> = (
  props,
) => {
  const [order, setOrder] = useState<any>({});
  const [messages, setMessages] = useState<any>([]);
  const [message, setMessage] = useState<string>('');
  const [currentPage, setCurrentPage] = useState<number>(1);
  const [total, setTotal] = useState<number>(0);
  const [pageSize, setPageSize] = useState<number>(20);
  const [fetched, setFetched] = useState<boolean>(false);
  const intl = useIntl();

  const getCommunicationMessages = async (page: number) => {
    const res = await pluginGetCommunicationMessages({
      comm_order_id: props.order.id,
      current: page,
      pageSize: pageSize,
    });
    setTotal(res.total || 0);
    setCurrentPage(page);
    const msgs = res.data || [];
    msgs.reverse();
    setMessages(msgs);
    setFetched(true);
  };
  const getOrderDetail = async () => {
    const res = await pluginGetCommunicationOrderInfo({
      id: props.order.id,
    });
    setOrder(res.data || null);
    setFetched(true);
  };

  useEffect(() => {
    setPageSize(20);
    getOrderDetail();
    getCommunicationMessages(1);
  }, []);

  const handleSubmitMessage = () => {
    pluginSendCommunicationMessage({
      comm_order_id: props.order.id,
      content: message,
      message_type: 'text',
    }).then(() => {
      setMessage('');
      getCommunicationMessages(1);
    });
  };

  const handleSelectImages = (field: string, rows: any) => {
    // eslint-disable-next-line guard-for-in
    for (let i in rows) {
      let exist = false;
      for (let j in order[field]) {
        if (rows[i].file_path === order[field][j]) {
          exist = true;
          break;
        }
      }
      if (!exist) {
        if (!order[field]) {
          order[field] = [];
        }
        order[field].push(rows[i].file_path);
      }
    }
    pluginSaveCommunicationOrderImages({
      id: order.id,
      field: field,
      images: order[field],
    }).then((res) => {
      message.indexOf(res.msg || 'success');
      setOrder(Object.assign({}, order));
    });
  };

  const handleDeleteImage = (field: string, index: number) => {
    order[field].splice(index, 1);
    pluginSaveCommunicationOrderImages({
      id: order.id,
      field: field,
      images: order[field],
    }).then((res) => {
      message.indexOf(res.msg || 'success');
      setOrder(Object.assign({}, order));
    });
  };

  const handleChangePage = (page: number) => {
    getCommunicationMessages(page);
  };

  const getStatusText = (status: number) => {
    let text = '';
    switch (status) {
      case 0:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.wait',
        });
        break;
      case 1:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.doing',
        });
        break;
      case 2:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.quote',
        });
        break;
      case 3:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.accepted',
        });
        break;
      case 4:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.finished',
        });
        break;
      case 5:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.rejected',
        });
        break;
      case -1:
        text = intl.formatMessage({
          id: 'plugin.order.communication.status.closed',
        });
        break;
    }

    return text;
  };

  return fetched ? (
    <ModalForm
      width={900}
      title={intl.formatMessage({ id: 'plugin.order.detail' })}
      open={props.open}
      layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      submitter={false}
    >
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.order.order-id' })}
        readonly
      >
        {order.id}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.order.status' })}
        readonly
      >
        {getStatusText(order.status)}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.order.create-time' })}
        readonly
      >
        {dayjs(order.created_time * 1000).format('YYYY-MM-DD HH:mm:ss')}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({
          id: 'plugin.order.communication.quote-price',
        })}
        readonly
      >
        {order.quote_price > 0 ? order.quote_price / 100 : '-'}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({
          id: 'plugin.order.communication.quote-content',
        })}
        readonly
      >
        {order.quote_content}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.user.phone' })}
        readonly
      >
        {order.phone}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({
          id: 'plugin.order.communication.event-date',
        })}
        readonly
      >
        {order.event_date}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.order.communication.budget' })}
        readonly
      >
        {order.title}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.order.communication.content' })}
        readonly
      >
        {order.content}
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({
          id: 'plugin.order.communication.inspiration-images',
        })}
        readonly
      >
        <Space size={16}>
          {order.images?.map((text: any, index: number) => (
            <div key={index} className="image-item">
              <Image
                key={index}
                height={105}
                preview={{ src: text }}
                src={text}
              />
              <span
                className="delete-icon"
                onClick={() => {
                  handleDeleteImage('images', index);
                }}
              >
                <DeleteOutlined />
              </span>
            </div>
          ))}
          <AttachmentSelect
            onSelect={(row) => handleSelectImages('images', row)}
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
        </Space>
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({
          id: 'plugin.order.communication.sketch-images',
        })}
        readonly
      >
        <Space size={16}>
          {order.sketch_images?.map((text: any, index: number) => (
            <div key={index} className="image-item">
              <Image
                key={index}
                height={105}
                preview={{ src: text }}
                src={text}
              />
              <span
                className="delete-icon"
                onClick={() => {
                  handleDeleteImage('sketch_images', index);
                }}
              >
                <DeleteOutlined />
              </span>
            </div>
          ))}
          <AttachmentSelect
            onSelect={(row) => handleSelectImages('sketch_images', row)}
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
        </Space>
      </ProFormText>
      <ProFormText
        label={intl.formatMessage({
          id: 'plugin.order.communication.finished-images',
        })}
        readonly
      >
        <Space size={16}>
          {order.finished_images?.map((text: any, index: number) => (
            <div key={index} className="image-item">
              <Image
                key={index}
                height={105}
                preview={{ src: text }}
                src={text}
              />
              <span
                className="delete-icon"
                onClick={() => {
                  handleDeleteImage('finished_images', index);
                }}
              >
                <DeleteOutlined />
              </span>
            </div>
          ))}
          <AttachmentSelect
            onSelect={(row) => handleSelectImages('finished_images', row)}
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
        </Space>
      </ProFormText>
      <Divider />
      {order.order_fields && (
        <div
          className="order-fields"
          style={{ border: '1px solid #eee', padding: '10px' }}
        >
          <h3>Custom Fields</h3>
          {order.order_fields?.map((field: any, index: number) => (
            <ProFormText key={index} label={field.name} readonly>
              {field.type === 'texts' ? (
                <div className="text-groups">
                  {order.order_extra?.[field.field_name]?.map(
                    (text: any, index: number) => (
                      <div className="text-group" key={index}>
                        <div className="text-key">{text.key}: </div>
                        <div className="text-value">{text.value}</div>
                      </div>
                    ),
                  )}
                </div>
              ) : field.type === 'image' ? (
                <Image
                  height={100}
                  preview={{
                    src: order.order_extra?.[field.field_name],
                  }}
                  src={order.order_extra?.[field.field_name]}
                />
              ) : field.type === 'file' ? (
                <a href={order.order_extra?.[field.field_name]}>
                  {order.order_extra?.[field.field_name]}
                </a>
              ) : field.type === 'images' ? (
                <Space size={16}>
                  {order.order_extra?.[field.field_name]?.map(
                    (text: any, index: number) => (
                      <Image
                        key={index}
                        height={100}
                        preview={{ src: text }}
                        src={text}
                      />
                    ),
                  )}
                </Space>
              ) : (
                <div>{order.order_extra?.[field.field_name]}</div>
              )}
            </ProFormText>
          ))}
        </div>
      )}
      <Divider>
        <FormattedMessage id="plugin.order.communication.messages" />
      </Divider>
      <List
        itemLayout="horizontal"
        dataSource={messages}
        renderItem={(item: any) => (
          <List.Item
            className={
              item.sender_type === 'admin'
                ? 'message-item message-self'
                : 'message-item'
            }
          >
            <List.Item.Meta
              avatar={<Avatar>{item.sender_name?.[0]}</Avatar>}
              title={item.sender_name}
              description={dayjs(item.created_time * 1000).format(
                'YYYY-MM-DD HH:mm:ss',
              )}
            />
            <div className="message-content">
              {item.message_type === 'image' ? (
                <div>
                  <Image
                    height={100}
                    preview={{ src: item.content }}
                    src={item.content}
                  />
                </div>
              ) : item.message_type === 'file' ? (
                <a href={item.content} target="_blank" rel="noreferrer">
                  {item.content}
                </a>
              ) : item.message_type === 'quote' ? (
                <div className="quote">
                  <div>{item.content}</div>
                  <div>{order.quote_content}</div>
                </div>
              ) : (
                <div dangerouslySetInnerHTML={{ __html: item.content }}></div>
              )}
            </div>
          </List.Item>
        )}
      />
      <Pagination
        align="center"
        defaultCurrent={currentPage}
        defaultPageSize={pageSize}
        total={total}
        showSizeChanger={false}
        onChange={handleChangePage}
      />

      <Divider>
        <FormattedMessage id="plugin.order.communication.message.reply" />
      </Divider>
      <Input.TextArea
        rows={4}
        placeholder={intl.formatMessage({
          id: 'plugin.order.communication.message.placeholder',
        })}
        value={message}
        onChange={(e) => {
          setMessage(e.target.value);
        }}
      />
      <div className="text-center mt-normal">
        <Button type="primary" onClick={() => handleSubmitMessage()}>
          <FormattedMessage id="plugin.order.communication.message.send" />
        </Button>
      </div>
    </ModalForm>
  ) : (
    <div></div>
  );
};

export default CommunicationOrderDetail;
