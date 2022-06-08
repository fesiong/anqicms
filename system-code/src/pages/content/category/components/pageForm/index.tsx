import React, { useEffect, useState } from 'react';
import { ModalForm, ProFormDigit, ProFormText, ProFormTextArea } from '@ant-design/pro-form';

import { saveCategory } from '@/services/category';
import WangEditor from '@/components/editor';
import { Upload, Image, message } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { uploadAttachment } from '@/services/attachment';
import AttachmentSelect from '@/components/attachment';

export type CategoryFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  type: number;
  visible: boolean;
  category: any;
};

const PageForm: React.FC<CategoryFormProps> = (props) => {
  const [content, setContent] = useState<string>('');
  const [categoryImages, setCategoryImages] = useState<string[]>([]);
  const [categoryLogo, setCategoryLogo] = useState<string>('');

  useEffect(() => {
    setCategoryImages(props.category?.images || [])
    setCategoryLogo(props.category?.logo || '')
  }, []);

  const onSubmit = async (values: any) => {
    let category = Object.assign(props.category, values);
    category.content = content;
    category.type = props.type;
    category.images = categoryImages;
    category.logo = categoryLogo;
    let res = await saveCategory(category);
    message.info(res.msg);

    props.onSubmit();
  };

  const handleSelectImages = (row: any) => {
    let exists = false;

    for (let i in categoryImages) {
      if (categoryImages[i] == row.logo) {
        exists = true;
        break;
      }
    }
    if (!exists) {
      categoryImages.push(row.logo);
    }
    setCategoryImages([].concat(categoryImages))
    message.success('上传完成');
  };

  const handleCleanImages = (index: number, e: any) => {
    e.stopPropagation();
    categoryImages.splice(index, 1);
    setCategoryImages([].concat(categoryImages))
  };

  const handleSelectLogo = (row: any) => {
    setCategoryLogo(row.logo);
    message.success('上传完成');
  };

  const handleCleanLogo = (e: any) => {
    e.stopPropagation();
    setCategoryLogo('')
  };

  return (
    <ModalForm
      width={800}
      title={props.category?.id ? '编辑页面' : '添加页面'}
      initialValues={props.category}
      visible={props.visible}
      layout="horizontal"
      onVisibleChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormText name="title" label="页面名称" />
      <ProFormText
        name="seo_title"
        label="SEO标题"
        placeholder="默认为页面名称，无需填写"
        extra="注意：如果你希望页面的title标签的内容不是页面名称，可以通过SEO标题设置"
      />
      <ProFormText
        name="keywords"
        label="标签关键词"
        extra="你可以单独设置关键词"
      />
      <ProFormText
        name="url_token"
        label="自定义URL"
        placeholder="默认会自动生成，无需填写"
        extra="注意：自定义URL只能填写字母、数字和下划线，不能带空格"
      />
      <ProFormTextArea name="description" label="页面简介" />
      <ProFormDigit name="sort" label="显示顺序" extra={'默认99，数字越小越靠前'} />
      <ProFormText name="template" label="页面模板" extra="页面默认值：page/detail.html" />
      <ProFormText label="Banner图">
            {categoryImages.length
              ? categoryImages.map((item: string, index: number) => (
                  <div className="ant-upload-item" key={index}>
                    <Image
                      preview={{
                        src: item,
                      }}
                      src={item}
                    />
                    <span className="delete" onClick={handleCleanImages.bind(this, index)}>
                      <DeleteOutlined />
                    </span>
                  </div>
                ))
              : null}
              <AttachmentSelect onSelect={ handleSelectImages } visible={false}>
              <div className="ant-upload-item">
                <div className='add'>
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>上传</div>
                </div>
              </div>
            </AttachmentSelect>
          </ProFormText>
          <ProFormText label="缩略图">
            {categoryLogo
              ?
              <div className="ant-upload-item">
                    <Image
                      preview={{
                        src: categoryLogo,
                      }}
                      src={categoryLogo}
                    />
                    <span className="delete" onClick={handleCleanLogo}>
                      <DeleteOutlined />
                    </span>
                  </div>
              : <AttachmentSelect onSelect={ handleSelectLogo } visible={false}>
              <div className="ant-upload-item">
                <div className='add'>
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>上传</div>
                </div>
              </div>
            </AttachmentSelect>}
          </ProFormText>
      <WangEditor
        className="mb-normal"
        setContent={async (html: string) => {
          setContent(html);
        }}
        content={props.category.content}
      />
    </ModalForm>
  );
};

export default PageForm;
