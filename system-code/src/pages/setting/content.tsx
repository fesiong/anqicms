import React, { useEffect, useState } from 'react';
import ProForm, {
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProFormRadio,
  ProFormFieldSet,
  ProFormGroup,
} from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Card, message, Upload } from 'antd';
import { uploadAttachment } from '@/services/attachment';
import { PlusOutlined } from '@ant-design/icons';
import { getSettingContent, saveSettingContent } from '@/services/setting';
import AttachmentSelect from '@/components/attachment';

const SettingContactFrom: React.FC<any> = (props) => {
  const [setting, setSetting] = useState<any>(null);
  const [defaultThumb, setDefaultThumb] = useState<string>('');
  const [resize_image, setResizeImage] = useState<number>(0);
  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await getSettingContent();
    let setting = res.data || null;
    setSetting(setting);
    console.log(setting)
    setDefaultThumb(setting?.default_thumb || '');
    setResizeImage(setting?.resize_image || 0);
  };

  const handleSelectLogo = (row: any) => {
    setDefaultThumb(row.logo);
    message.success('上传完成');
  };

  const onSubmit = async (values: any) => {
    values.default_thumb = defaultThumb;
    values.filter_outlink = Number(values.filter_outlink);
    values.remote_download = Number(values.remote_download);
    values.resize_image = Number(values.resize_image);
    values.resize_width = Number(values.resize_width);
    values.thumb_crop = Number(values.thumb_crop);
    values.thumb_width = Number(values.thumb_width);
    values.thumb_height = Number(values.thumb_height);

    saveSettingContent(values)
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
        {setting && (
          <ProForm initialValues={setting} onFinish={onSubmit} title="联系方式设置">
            <ProFormRadio.Group
              name="remote_download"
              label="下载远程图片"
              options={[
                {
                  value: 0,
                  label: '不下载',
                },
                {
                  value: 1,
                  label: '下载',
                },
              ]}
            />
            <ProFormRadio.Group
              name="filter_outlink"
              label="自动过滤外链"
              options={[
                {
                  value: 0,
                  label: '不过滤',
                },
                {
                  value: 1,
                  label: '过滤',
                },
              ]}
            />
            <ProFormRadio.Group
              name="resize_image"
              label="自动压缩大图"
              fieldProps={{
                onChange: (e: any) => {
                  setResizeImage(e.target.value);
                },
              }}
              options={[
                {
                  value: 0,
                  label: '不压缩',
                },
                {
                  value: 1,
                  label: '压缩',
                },
              ]}
            />
            {resize_image == 1 && (
              <ProFormText
                name="resize_width"
                label="压缩到指定宽度"
                width="lg"
                placeholder="默认：800"
                fieldProps={{
                  suffix: '像素',
                }}
              />
            )}
            <ProFormRadio.Group
              name="thumb_crop"
              label="缩略图方式"
              options={[
                {
                  value: 0,
                  label: '按最长边等比缩放',
                },
                {
                  value: 1,
                  label: '按最长边补白',
                },
                {
                  value: 3,
                  label: '按最短边裁剪',
                },
              ]}
            />
            <ProFormGroup label="缩略图尺寸">
              <ProFormText
                name="thumb_width"
                width="sm"
                fieldProps={{
                  suffix: '像素宽',
                }}
              />
              ×
              <ProFormText
                name="thumb_height"
                width="sm"
                fieldProps={{
                  suffix: '像素高',
                }}
              />
            </ProFormGroup>
            <ProFormText
              label="默认缩略图"
              width="lg"
              extra="如果文章没有缩略图，继续调用将会使用默认缩略图代替"
            >
              <AttachmentSelect onSelect={ handleSelectLogo } visible={false}>
                <div className="ant-upload-item">
                {defaultThumb ? (
                  <img src={defaultThumb} style={{ width: '100%' }} />
                ) : (
                  <div className='add'>
                    <PlusOutlined />
                    <div style={{ marginTop: 8 }}>上传</div>
                  </div>
                )}
                </div>
              </AttachmentSelect>
            </ProFormText>
          </ProForm>
        )}
      </Card>
    </PageHeaderWrapper>
  );
};

export default SettingContactFrom;
