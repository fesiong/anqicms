import { getCategories, pluginSaveKeyword } from '@/services';
import {
  ModalForm,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

export type KeywordFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  editingKeyword: any;
};

const KeywordForm: React.FC<KeywordFormProps> = (props) => {
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<any[]>([]);
  const intl = useIntl();

  useEffect(() => {
    getCategories({ type: 1 }).then((res) => {
      setCategories(res.data || []);
    });
  }, []);

  const onSubmit = async (values: any) => {
    if (loading) return;
    setLoading(true);
    const hide = message.loading(
      {
        content: intl.formatMessage({ id: 'setting.system.submitting' }),
        key: 'loading',
      },
      0,
    );
    let editingKeyword = Object.assign(props.editingKeyword, values);
    await pluginSaveKeyword(editingKeyword);
    hide();
    setLoading(false);
    props.onSubmit();
  };

  return (
    <ModalForm
      width={800}
      title={
        props.editingKeyword?.id
          ? intl.formatMessage({ id: 'plugin.keyword.edit' })
          : intl.formatMessage({ id: 'plugin.keyword.add' })
      }
      initialValues={props.editingKeyword}
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
      {props.editingKeyword?.id > 0 ? (
        <ProFormText
          name="title"
          label={intl.formatMessage({ id: 'plugin.keyword.title' })}
        />
      ) : (
        <ProFormTextArea
          fieldProps={{
            rows: 5,
          }}
          name="title"
          label={intl.formatMessage({ id: 'plugin.keyword.title' })}
          placeholder={intl.formatMessage({
            id: 'plugin.keyword.title.placeholder',
          })}
        />
      )}
      <ProFormSelect
        label={intl.formatMessage({ id: 'plugin.keyword.archive-category' })}
        name="category_id"
        width="lg"
        options={[
          {
            title: intl.formatMessage({
              id: 'content.please-select',
            }),
            value: 0,
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
          }))}
        fieldProps={{
          showSearch: true,
          filterOption: (input: string, option: any) =>
            (option?.title ?? option?.label)
              .toLowerCase()
              .includes(input.toLowerCase()),
        }}
      />
    </ModalForm>
  );
};

export default KeywordForm;
