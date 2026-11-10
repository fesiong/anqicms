import NewAiEditor from '@/components/newAiEditor';
import {
  pluginGetMaterialCategories,
  pluginSaveMaterial,
} from '@/services/plugin/material';
import { ModalForm, ProFormSelect } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import React, { useState } from 'react';

export type MaterialFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingMaterial: any;
};

const MaterialForm: React.FC<MaterialFormProps> = (props) => {
  const [content, setContent] = useState<string>('');
  const intl = useIntl();

  const onSubmit = async (values: any) => {
    let editingMaterial = Object.assign(props.editingMaterial, values);
    editingMaterial.content = content;
    await pluginSaveMaterial(editingMaterial);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={intl.formatMessage({ id: 'plugin.material.edit' })}
      initialValues={props.editingMaterial}
      open={props.open}
      //layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormSelect
        label={intl.formatMessage({ id: 'plugin.material.category.title' })}
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
          optionItemRender(item: any) {
            return item.title;
          },
        }}
      />
      <NewAiEditor
        className="mb-normal"
        setContent={async (html: string) => {
          setContent(html);
        }}
        key="content"
        field="content"
        content={props.editingMaterial.content}
        ref={null}
      />
    </ModalForm>
  );
};

export default MaterialForm;
