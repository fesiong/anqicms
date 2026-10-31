import {
  ModalForm,
  ProFormCheckbox,
  ProFormDateTimePicker,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import React, { useEffect, useState } from 'react';

import {
  getArchiveInfo,
  getCategories,
  getSettingContent,
  getTags,
  saveArchive,
} from '@/services';
import { setStore } from '@/utils/store';
import { history, useIntl } from '@umijs/max';
import { Col, Modal, Row, message } from 'antd';
import dayjs from 'dayjs';

export type QuickEditFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  archive: any;
};

const QuickEditForm: React.FC<QuickEditFormProps> = (props) => {
  const [searchedTags, setSearchedTags] = useState<any>({});
  const [contentSetting, setContentSetting] = useState<any>({});
  const [archive, setArchive] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const intl = useIntl();

  const getArchive = async (id: number) => {
    const res = await getArchiveInfo({
      id: id,
    });
    const data = res.data || { extra: {}, flag: null };
    data.flag = data.flag?.split(',') || [];
    data.created_moment = dayjs(data.created_time * 1000);
    data.tags = data.tags?.map((tag: any) => tag.title);
    setArchive(data);
    setFetched(true);
  };

  useEffect(() => {
    getArchive(props.archive.id);
    getSettingContent().then((res) => {
      setContentSetting(res.data || {});
    });
  }, []);

  const onSubmit = async (values: any) => {
    let postData = Object.assign(archive, values);
    if (postData.title === '') {
      message.error(intl.formatMessage({ id: 'content.title.required' }));
      return;
    }
    let categoryIds = [];
    let categoryId = 0;
    if (typeof values.category_ids === 'number') {
      // 单分类
      categoryId = Number(values.category_ids);
    } else {
      for (let i in values.category_ids) {
        if (values.category_ids[i] > 0) {
          categoryIds.push(values.category_ids[i]);
        }
      }
      if (categoryIds.length > 0) {
        categoryId = categoryIds[0];
      }
    }
    if (categoryId === 0) {
      message.error(intl.formatMessage({ id: 'content.category.required' }));
      return;
    }
    postData.category_id = categoryId;
    postData.category_ids = categoryIds;
    if (typeof postData.flag === 'object') {
      postData.flag = postData.flag.join(',');
    }
    // 保留，防止低版本出问题
    postData.content = archive.data?.content || '';
    // 标记为快速保存
    postData.quick_save = true;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    const res = await saveArchive(postData);
    hide();
    if (res.code !== 0) {
      message.error(res.msg);
    } else {
      // 设置最近更新过的文档
      setStore('latest_update', {
        id: res.data.id,
        timestamp: new Date().getTime() / 1000,
      });
      message.success(res.msg);
    }
    props.onSubmit();
  };

  const onChangeTagInput = (e: any) => {
    const value = e.target?.value || '';
    getTags({
      type: 1,
      title: value,
      pageSize: 10,
    }).then((res) => {
      const data = res.data || [];
      const result: any = {};
      for (const item of data) {
        result[item.title] = item.title;
      }
      setSearchedTags(result);
    });
  };

  return (
    fetched && (
      <ModalForm
        width={900}
        title={intl.formatMessage({ id: 'content.action.quick-edit' })}
        initialValues={archive}
        open={props.open}
        layout="vertical"
        onOpenChange={(flag) => {
          if (!flag) {
            props.onCancel(flag);
          }
        }}
        onFinish={async (values) => {
          onSubmit(values);
        }}
      >
        <Row gutter={16}>
          <Col span={12}>
            <ProFormText
              name="title"
              label={intl.formatMessage({ id: 'content.title.name' })}
            />
            <ProFormText
              name="keywords"
              label={intl.formatMessage({ id: 'content.keywords.name' })}
            />
            <ProFormTextArea
              name="description"
              label={intl.formatMessage({ id: 'content.description.name' })}
            />
            <ProFormCheckbox.Group
              name="flag"
              label={intl.formatMessage({ id: 'content.flag.name' })}
              valueEnum={{
                h: intl.formatMessage({ id: 'content.flag.h' }),
                c: intl.formatMessage({ id: 'content.flag.c' }),
                f: intl.formatMessage({ id: 'content.flag.f' }),
                a: intl.formatMessage({ id: 'content.flag.a' }),
                s: intl.formatMessage({ id: 'content.flag.s' }),
                b: intl.formatMessage({ id: 'content.flag.b' }),
                p: intl.formatMessage({ id: 'content.flag.p' }),
                j: intl.formatMessage({ id: 'content.flag.j' }),
              }}
            />
          </Col>
          <Col span={12}>
            <ProFormSelect
              label={intl.formatMessage({ id: 'content.category.name' })}
              showSearch
              name="category_ids"
              mode={contentSetting.multi_category === 1 ? 'multiple' : 'single'}
              request={async () => {
                const res = await getCategories({ type: 1 });
                const categories = res.data || [];
                if (categories.length === 0) {
                  Modal.error({
                    title: intl.formatMessage({ id: 'content.category.error' }),
                    onOk: () => {
                      history.push('/archive/category');
                    },
                  });
                }
                return categories;
              }}
              fieldProps={{
                fieldNames: {
                  label: 'title',
                  value: 'id',
                },
                optionItemRender(item: any) {
                  return (
                    <div
                      dangerouslySetInnerHTML={{
                        __html: item.spacer + item.title,
                      }}
                    ></div>
                  );
                },
              }}
            />
            <ProFormSelect
              label={intl.formatMessage({ id: 'content.tag.name' })}
              mode="tags"
              name="tags"
              valueEnum={searchedTags}
              placeholder={intl.formatMessage({
                id: 'content.tag.placeholder',
              })}
              fieldProps={{
                tokenSeparators: [',', '，'],
                onInputKeyDown: onChangeTagInput,
                onFocus: onChangeTagInput,
              }}
              extra={intl.formatMessage({ id: 'content.tag.placeholder' })}
            />
            <ProFormText
              label={intl.formatMessage({ id: 'content.url-token.name' })}
              name="url_token"
              placeholder={intl.formatMessage({
                id: 'content.url-token.placeholder',
              })}
              extra={intl.formatMessage({ id: 'content.url-token.tips' })}
            />
            <ProFormDateTimePicker
              name="created_moment"
              label={intl.formatMessage({ id: 'content.create-time.name' })}
              placeholder={intl.formatMessage({
                id: 'content.url-token.placeholder',
              })}
              extra={intl.formatMessage({
                id: 'content.create-time.description',
              })}
              transform={(value) => {
                return {
                  created_time: value ? dayjs(value).unix() : 0,
                };
              }}
            />
          </Col>
        </Row>
      </ModalForm>
    )
  );
};

export default QuickEditForm;
