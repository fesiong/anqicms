import MarkdownEditor from '@/components/markdown';
import { anqiShareTemplate, anqiUpload } from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Modal, RadioChangeEvent, Upload, message } from 'antd';
import React, { useRef, useState } from 'react';

export type TemplateShareProps = {
  children: any;
  designInfo: any;
  canShare: boolean;
  templateId: number;
  onFinished?: () => void;
};

const TemplateShare: React.FC<TemplateShareProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const [pcThumb, setPcThumb] = useState<string>('');
  const [mobileThumb, setMobileThumb] = useState<string>('');
  const [previewImages, setPreviewImages] = useState<string[]>([]);
  const [onlyTemplate, setOnlyTemplate] = useState<boolean>(false);
  const [description, setDescription] = useState<string>('');
  const formRef = useRef<ProFormInstance>();
  const intl = useIntl();

  const confirmShareTemplate = async (values: any) => {
    Modal.confirm({
      title:
        props.templateId > 0
          ? intl.formatMessage({ id: 'design.share.confirm-replace' })
          : intl.formatMessage({ id: 'design.share.confirm-new' }),
      content: (
        <div>
          {props.templateId > 0
            ? intl.formatMessage({ id: 'design.share.confirm-replace.content' })
            : intl.formatMessage({ id: 'design.share.confirm-new.content' })}
        </div>
      ),
      onOk: async () => {
        const postData = values;
        postData.package = props.designInfo.package;
        postData.template_id = props.designInfo.templateId;
        postData.price = Math.floor(postData.price * 100);
        postData.auto_backup = !!postData.auto_backup;
        postData.pc_thumb = pcThumb;
        postData.mobile_thumb = mobileThumb;
        postData.preview_images = previewImages;
        postData.only_template = onlyTemplate;
        postData.description = description;

        anqiShareTemplate(postData).then((res) => {
          if (res.code === 0) {
            message.info(
              res.msg ||
                intl.formatMessage({ id: 'setting.system.submit-success' }),
            );
            setVisible(false);
            props.onFinished?.();
          } else {
            message.info(res.msg);
          }
        });
      },
    });
  };

  const handleUploadImage = (field: string, e: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );

    let formData = new FormData();
    formData.append('file', e.file);
    anqiUpload(formData)
      .then((res) => {
        if (res.code !== 0) {
          message.info(res.msg);
        } else {
          message.info(
            res.msg ||
              intl.formatMessage({ id: 'setting.system.upload-success' }),
          );
          if (field === 'pc_thumb') {
            setPcThumb(res.data.file_path || res.data.logo);
          } else if (field === 'mobile_thumb') {
            setMobileThumb(res.data.file_path || res.data.logo);
          } else {
            previewImages.push(res.data.file_path || res.data.logo);
            setPreviewImages(([] as string[]).concat(...previewImages));
          }
        }
      })
      .finally(() => {
        hide();
      });
  };

  const handleChangeOption = (e: RadioChangeEvent) => {
    setOnlyTemplate(e.target.value);
  };

  return (
    <>
      <div
        onClick={() => {
          if (!props.canShare) {
            message.info(intl.formatMessage({ id: 'design.share.nologin' }));
            return;
          }
          formRef.current?.setFieldsValue(props.designInfo);
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <ModalForm
        width={800}
        title={intl.formatMessage({ id: 'design.share.share' })}
        open={visible}
        formRef={formRef}
        modalProps={{
          onCancel: () => {
            setVisible(false);
          },
        }}
        layout="horizontal"
        onFinish={async (values) => {
          confirmShareTemplate(values);
        }}
      >
        <ProFormText
          name="name"
          label={intl.formatMessage({ id: 'design.detail.template-name' })}
          extra={intl.formatMessage({
            id: 'design.detail.template-name.example',
          })}
        />
        <ProFormRadio.Group
          label={intl.formatMessage({ id: 'content.category.name' })}
          name="category_id"
          options={[
            {
              label: intl.formatMessage({
                id: 'design.share.category.foreign-trade',
              }),
              value: 36,
            },
            {
              label: intl.formatMessage({
                id: 'design.share.category.chinese',
              }),
              value: 37,
            },
            {
              label: intl.formatMessage({
                id: 'design.share.category.free',
              }),
              value: 38,
            },
          ]}
        />
        {props.templateId > 0 && (
          <ProFormRadio.Group
            name="only_template"
            initialValue={false}
            label={intl.formatMessage({ id: 'design.share.only-template' })}
            options={[
              {
                value: false,
                label: intl.formatMessage({
                  id: 'design.share.only-template.no',
                }),
              },
              {
                value: true,
                label: intl.formatMessage({
                  id: 'design.share.only-template.yes',
                }),
              },
            ]}
            fieldProps={{
              onChange: handleChangeOption,
            }}
            extra={intl.formatMessage({
              id: 'design.share.only-template.tips',
            })}
          />
        )}
        <ProFormText
          name="version"
          label={intl.formatMessage({ id: 'design.share.version' })}
          placeholder={intl.formatMessage({
            id: 'design.share.version.placeholder',
          })}
        />
        {!onlyTemplate && (
          <>
            <ProFormDigit
              name="price"
              label={intl.formatMessage({ id: 'design.share.price' })}
              addonAfter={intl.formatMessage({
                id: 'design.share.price.suffix',
              })}
              extra={intl.formatMessage({
                id: 'design.share.price.description',
              })}
            />
            <ProFormRadio.Group
              name="auto_backup"
              label={intl.formatMessage({ id: 'design.share.example-data' })}
              options={[
                {
                  value: 0,
                  label: intl.formatMessage({
                    id: 'design.share.example-data.no',
                  }),
                },
                {
                  value: 1,
                  label: intl.formatMessage({
                    id: 'design.share.example-data.yes',
                  }),
                },
              ]}
              extra={intl.formatMessage({
                id: 'design.share.example-data.description',
              })}
            />
            <ProFormText
              name="author"
              label={intl.formatMessage({ id: 'design.share.author' })}
            />
            <ProFormText
              name="homepage"
              label={intl.formatMessage({ id: 'design.share.homepage' })}
            />
            <MarkdownEditor
              className="mb-normal"
              setContent={async (html) => {
                setDescription(html);
              }}
              content={description}
              ref={null}
            />
            <ProFormText
              label={intl.formatMessage({ id: 'design.share.thumb.pc' })}
            >
              <Upload
                name="file"
                multiple
                showUploadList={false}
                accept=".jpg,.jpeg,.png,.gif,.webp,.bmp"
                customRequest={(e) => {
                  handleUploadImage('pc_thumb', e);
                }}
              >
                <div className="ant-upload-item">
                  {pcThumb ? (
                    <img src={pcThumb} style={{ width: '100%' }} />
                  ) : (
                    <div className="add">
                      <PlusOutlined />
                      <div style={{ marginTop: 8 }}>
                        <FormattedMessage id="setting.system.upload" />
                      </div>
                    </div>
                  )}
                </div>
              </Upload>
            </ProFormText>
            <ProFormText
              label={intl.formatMessage({ id: 'design.share.thumb.m' })}
            >
              <Upload
                name="file"
                multiple
                showUploadList={false}
                accept=".jpg,.jpeg,.png,.gif,.webp,.bmp"
                customRequest={(e) => {
                  handleUploadImage('mobile_thumb', e);
                }}
              >
                <div className="ant-upload-item">
                  {mobileThumb ? (
                    <img src={mobileThumb} style={{ width: '100%' }} />
                  ) : (
                    <div className="add">
                      <PlusOutlined />
                      <div style={{ marginTop: 8 }}>
                        <FormattedMessage id="setting.system.upload" />
                      </div>
                    </div>
                  )}
                </div>
              </Upload>
            </ProFormText>
            <ProFormText
              label={intl.formatMessage({ id: 'design.share.preview' })}
              extra={intl.formatMessage({
                id: 'design.share.preview.description',
              })}
            >
              <Upload
                name="file"
                multiple
                showUploadList={false}
                accept=".jpg,.jpeg,.png,.gif,.webp,.bmp"
                customRequest={(e) => {
                  handleUploadImage('preview_images', e);
                }}
              >
                <div className="ant-upload-item">
                  <div className="add">
                    <PlusOutlined />
                    <div style={{ marginTop: 8 }}>
                      <FormattedMessage id="setting.system.upload" />
                    </div>
                  </div>
                </div>
              </Upload>
              {previewImages.map((item: string, index: number) => (
                <div className="ant-upload-item" key={index}>
                  <img src={item} style={{ width: '100%' }} />
                </div>
              ))}
            </ProFormText>
          </>
        )}
      </ModalForm>
    </>
  );
};

export default TemplateShare;
