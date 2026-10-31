import { getCategories } from '@/services';
import { saveTag } from '@/services/tag';
import {
  ModalForm,
  ProFormSelect,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

export type BatchFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
};

const BatchForm: React.FC<BatchFormProps> = (props) => {
  const [loading, setLoading] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const intl = useIntl();

  useEffect(() => {
    getCategories({ type: 1 }).then((res) => {
      setCategories(res.data || []);
    });
  }, []);

  const onSubmit = async (values: any) => {
    if (loading) {
      return;
    }
    setLoading(true);
    let done = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    let tags = values.tags.split('\n');
    for (let i in tags) {
      if (tags[i].length === 0) {
        continue;
      }
      await saveTag({
        title: tags[i],
        category_id: Number(values.category_id),
      });
    }
    setLoading(false);
    done();
    props.onSubmit();
  };

  return (
    <ModalForm
      width={600}
      title={intl.formatMessage({ id: 'content.tag.batch-add' })}
      open={props.open}
      layout="horizontal"
      modalProps={{
        okButtonProps: {
          loading: loading,
        },
      }}
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
        label={intl.formatMessage({ id: 'content.category.name' })}
        showSearch
        name="category_id"
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
      <ProFormTextArea
        name="tags"
        label={intl.formatMessage({ id: 'content.tags.name' })}
        placeholder={intl.formatMessage({ id: 'content.tags.placeholder' })}
        fieldProps={{ rows: 15 }}
      />
    </ModalForm>
  );
};

export default BatchForm;
