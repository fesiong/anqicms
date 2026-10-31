import { addUrlToAttachment, getCategories } from '@/services';
import { DeleteOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormInstance,
  ProFormSelect,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { Button, Image, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import './index.less';

export type AttachmentAddUrlProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: () => void;
  open: boolean;
  intl?: any;
};

const AttachmentAddUrl: React.FC<AttachmentAddUrlProps> = (props) => {
  const formRef = useRef<ProFormInstance>();
  const [urls, setUrls] = useState<string[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [selectedCategory, setSelectedCategory] = useState<number>(0);

  useEffect(() => {}, []);

  const parseUrls = (value: string) => {
    const tmpUrls = value.split('\n');
    const newUrls: string[] = [];
    tmpUrls.forEach((item) => {
      if (item.trim() && (item.startsWith('http') || item.startsWith('//'))) {
        newUrls.push(item.trim());
      }
    });
    setUrls(newUrls);
  };

  const handleCleanImages = (index: number, e: any) => {
    e.stopPropagation();
    urls.splice(index, 1);
    setUrls([...urls]);
  };

  const handleSaveToAttachment = () => {
    if (!urls.length) {
      return;
    }
    Modal.confirm({
      zIndex: 9999,
      title: props.intl?.formatMessage({
        id: 'content.attachment.add-url.confirm',
      }),
      icon: null,
      width: 600,
      onOk: async () => {
        setLoading(true);
        addUrlToAttachment({
          urls: urls,
          category_id: selectedCategory,
        })
          .then(() => {
            message.success(
              props.intl?.formatMessage({
                id: 'content.attachment.add-url.success',
              }),
            );
            props.onSubmit();
          })
          .finally(() => {
            setLoading(false);
          });
      },
    });
  };

  return (
    <>
      <Modal
        width={800}
        title={props.intl?.formatMessage({ id: 'content.attachment.add-url' })}
        open={props.open}
        onCancel={() => {
          props.onCancel();
        }}
        footer={null}
      >
        <ProForm layout="horizontal" formRef={formRef} submitter={false}>
          <ProFormSelect
            name="category_id"
            label={props.intl?.formatMessage({
              id: 'content.category.name',
            })}
            request={async () => {
              let res = await getCategories({});
              let categories = res.data || [];
              categories = [
                {
                  id: 0,
                  title: props.intl?.formatMessage({
                    id: 'content.attachment.unclassified',
                  }),
                  status: 1,
                },
              ]
                .concat(categories)
                .map((cat: any) => ({
                  title: cat.title,
                  label: (
                    <div title={cat.title}>
                      {cat.parents?.length > 0 ? (
                        <span className="text-muted">
                          {cat.parents
                            ?.map((parent: any) => parent.title)
                            .join(' > ')}
                          {' > '}
                        </span>
                      ) : (
                        ''
                      )}
                      {cat.title}
                    </div>
                  ),
                  value: cat.id,
                  disabled: cat.status !== 1,
                }));
              return categories;
            }}
            fieldProps={{
              onChange: (value) => {
                setSelectedCategory(value as number);
              },
              showSearch: true,
              filterOption: (input: string, option: any) =>
                (option?.title ?? option?.label)
                  .toLowerCase()
                  .includes(input.toLowerCase()),
            }}
          />
          <ProFormTextArea
            name="urls"
            placeholder={props.intl?.formatMessage({
              id: 'content.attachment.add-url.placeholder',
            })}
            fieldProps={{
              rows: 10,
              onChange: (e) => {
                const value = e.target.value;
                parseUrls(value);
              },
            }}
          />
          {urls.length
            ? urls.map((item: string, index: number) => {
                const splitItem = item.split('|');
                let fileName = '';
                let fileUrl = splitItem[0].trim();
                if (splitItem.length > 1) {
                  fileName = splitItem[1].trim();
                }
                return (
                  <div className="ant-upload-item" key={index}>
                    <Image
                      preview={{
                        src: fileUrl,
                      }}
                      src={fileUrl}
                    />
                    {fileName && <span className="file-name">{fileName}</span>}
                    <span
                      className="delete"
                      onClick={handleCleanImages.bind(this, index)}
                    >
                      <DeleteOutlined />
                    </span>
                  </div>
                );
              })
            : null}
          <Space
            className="mt-normal"
            align="center"
            style={{ width: '100%', justifyContent: 'center' }}
          >
            <Button
              type="primary"
              disabled={!urls.length || loading}
              onClick={() => handleSaveToAttachment()}
            >
              {props.intl?.formatMessage({
                id: 'content.attachment.add-url.submit',
              })}
            </Button>
            <Button
              onClick={() => {
                props.onCancel();
              }}
            >
              {props.intl?.formatMessage({
                id: 'content.attachment.add-url.cancel',
              })}
            </Button>
          </Space>
        </ProForm>
      </Modal>
    </>
  );
};

export default AttachmentAddUrl;
