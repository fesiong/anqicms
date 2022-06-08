import React, { useState } from 'react';
import { ModalForm, ProFormSelect } from '@ant-design/pro-form';
import { pluginGetMaterialCategories, pluginSaveMaterial } from '@/services/plugin/material';
import WangEditor from '@/components/editor';

export type MaterialFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  visible: boolean;
  editingMaterial: any;
};

const MaterialForm: React.FC<MaterialFormProps> = (props) => {
  const [content, setContent] = useState<string>('');

  const onSubmit = async (values: any) => {
    let editingMaterial = Object.assign(props.editingMaterial, values);
    editingMaterial.content = content;
    let res = await pluginSaveMaterial(editingMaterial);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={'编辑内容素材'}
      initialValues={props.editingMaterial}
      visible={props.visible}
      //layout="horizontal"
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
        label="素材板块"
        name="category_id"
        width="lg"
        request={async () => {
          let res = await pluginGetMaterialCategories({});
          return res.data || [];
        }}
        fieldProps={{
          fieldNames: {
            label: 'title',
            value: 'id',
          },
          optionItemRender(item) {
            return item.title;
          },
        }}
      />
      <WangEditor
        className="mb-normal"
        setContent={async (html: string) => {
          setContent(html);
        }}
        content={props.editingMaterial.content}
      />
    </ModalForm>
  );
};

export default MaterialForm;
