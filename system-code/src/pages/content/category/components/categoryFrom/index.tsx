import React, { useEffect, useState } from 'react';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';

import { getCategories, saveCategory } from '@/services';
import { Collapse, message, Upload, Image } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import WangEditor from '@/components/editor';
import AttachmentSelect from '@/components/attachment';

export type CategoryFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  type: number;
  visible: boolean;
  category: any;
  modules: any[],
};

const CategoryForm: React.FC<CategoryFormProps> = (props) => {
  const [content, setContent] = useState<string>('');
  const [categoryImages, setCategoryImages] = useState<string[]>([]);
  const [categoryLogo, setCategoryLogo] = useState<string>('');
  const [currentModule, setCurrentModule] = useState<any>({});

  useEffect(() => {
    setCategoryImages(props.category?.images || [])
    setCategoryLogo(props.category?.logo || '')
    let moduleId = props.category?.module_id || 1;
    changeModule(moduleId)
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

  const changeModule = (e: any) => {
    for (let item of props.modules) {
      if (item.id == e) {
        setCurrentModule(item);
        break
      }
    }
  }

  return (
    <ModalForm
      width={800}
      title={props.category?.id ? '编辑分类' : '添加分类'}
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
      <ProFormSelect
        label="内容模型"
        name="module_id"
        width="lg"
        request={async () => {
          return props.modules;
        }}
        readonly={(props.category?.id || props.category?.module_id > 0) ? true : false}
        fieldProps={{
          fieldNames: {
            label: 'title',
            value: 'id',
          },
          onChange: (e) => {
            changeModule(e)
          }
        }}
      />
      <ProFormSelect
        label="上级分类"
        name="parent_id"
        width="lg"
        request={async () => {
          let res = await getCategories({ type: props.type });
          let categories = res.data || [];
          // 排除自己
          if (props.category.id) {
            let tmpCategory = [];
            for (let i in categories) {
              if (
                categories[i].id == props.category.id ||
                categories[i].parent_id == props.category.id ||
                categories[i].module_id != props.category.module_id
              ) {
                continue;
              }
              tmpCategory.push(categories[i]);
            }
            categories = tmpCategory;
          }
          categories = [{ id: 0, title: '顶级分类', spacer: '' }].concat(categories);
          return categories;
        }}
        readonly={(props.category?.id || props.category?.module_id > 0) ? false : true}
        fieldProps={{
          fieldNames: {
            label: 'title',
            value: 'id',
          },
          optionItemRender(item) {
            return (
              <div dangerouslySetInnerHTML={{ __html: (item.spacer || '') + item.title }}></div>
            );
          },
        }}
      />
      <ProFormText name="title" label="分类名称" />
      <ProFormTextArea name="description" label="分类简介" />
      <Collapse>
        <Collapse.Panel header="其他参数" key="1">
          <ProFormDigit name="sort" label="显示顺序" extra={'默认99，数字越小越靠前'} />
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
          <ProFormText
            name="template"
            label="分类模板"
            extra={
              <div>分类默认值：{currentModule.table_name}/list.html</div>
            }
          />
          <ProFormRadio.Group
              name="is_inherit"
              label="应用到子分类"
              options={[
                {
                  value: 0,
                  label: '不应用',
                },
                {
                  value: 1,
                  label: '应用',
                },
              ]}
              extra='如果设置了自定义分类模板，可以选择应用到所有子分类，或者仅对当前分类生效'
            />
          <ProFormText
            name="detail_template"
            label="文档模板"
            extra={
              <div>文档模板默认值：{currentModule.table_name}/detail.html</div>
            }
          />
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
        </Collapse.Panel>
      </Collapse>
    </ModalForm>
  );
};

export default CategoryForm;
