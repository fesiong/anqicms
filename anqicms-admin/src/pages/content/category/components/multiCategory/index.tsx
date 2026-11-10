import { getCategories, saveCategory } from '@/services';
import {
  ModalForm,
  ProFormInstance,
  ProFormSelect,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

export type MultiCategoryProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  type: number;
  open: boolean;
  category: any;
  modules: any[];
};

const MultiCategory: React.FC<MultiCategoryProps> = (props) => {
  const formRef = React.createRef<ProFormInstance>();
  const [currentModule, setCurrentModule] = useState<any>({});
  const [categories, setCategories] = useState<any[]>([]);
  const intl = useIntl();

  const changeModule = (e: any) => {
    for (let item of props.modules) {
      if (item.id === e) {
        setCurrentModule(item);
        break;
      }
    }
    getCategories({}).then((res) => {
      setCategories(res.data || []);
    });
  };

  useEffect(() => {
    let moduleId = props.category?.module_id || 1;
    changeModule(moduleId);
  }, []);

  const onSubmit = async (values: any) => {
    let categoryData = {
      parent_id: values.parent_id,
      module_id: values.module_id,
      type: props.type,
      title: '',
      url_token: '',
      status: 1,
    };
    let splitData = values.inputs.split('\n');
    for (let item of splitData) {
      let names = item.trim().split('|');
      categoryData.title = names[0];
      categoryData.url_token = '';
      if (names.length > 1) {
        categoryData.url_token = names[1];
      }
      if (categoryData.title.length > 0) {
        let res = await saveCategory(categoryData);
        message.info(res.msg);
      }
    }
    props.onSubmit();
  };

  return (
    <ModalForm
      formRef={formRef}
      width={600}
      title={intl.formatMessage({ id: 'content.category.batch-add' })}
      initialValues={props.category}
      open={props.open}
      layout="horizontal"
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
        label={intl.formatMessage({ id: 'content.module.name' })}
        name="module_id"
        width="lg"
        request={async () => {
          return props.modules;
        }}
        readonly={
          props.category?.id || props.category?.module_id > 0 ? true : false
        }
        fieldProps={{
          fieldNames: {
            label: 'name',
            value: 'id',
          },
          onChange: (e) => {
            changeModule(e);
          },
        }}
      />
      <ProFormSelect
        label={intl.formatMessage({ id: 'content.category.parent' })}
        name="parent_id"
        width="lg"
        options={[
          {
            id: 0,
            title: intl.formatMessage({
              id: 'content.category.top',
            }),
            status: 1,
          },
        ]
          .concat(
            categories.filter((item) =>
              props.category.id > 0
                ? item.module_id === props.category.module_id &&
                  item.id !== props.category.id &&
                  item.parent_id !== props.category.id
                : true,
            ),
          )
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
        readonly={
          props.category?.id || props.category?.module_id > 0 ? false : true
        }
      />
      <ProFormTextArea
        name="inputs"
        width="lg"
        label={intl.formatMessage({ id: 'content.category.batch-name' })}
        placeholder={intl.formatMessage({
          id: 'content.category.batch-name.placeholder',
        })}
        fieldProps={{
          rows: 10,
        }}
        extra={
          <div>
            <div>
              <FormattedMessage id="content.category.batch-name.tips1" />
            </div>
            <div>
              <FormattedMessage id="content.category.batch-name.tips2" />
            </div>
            <div>
              <code>
                <div>
                  <FormattedMessage id="content.category.batch-name.tips3" />
                  <br />
                  <FormattedMessage id="content.category.batch-name.tips4" />
                </div>
              </code>
            </div>
          </div>
        }
      />
    </ModalForm>
  );
};

export default MultiCategory;
