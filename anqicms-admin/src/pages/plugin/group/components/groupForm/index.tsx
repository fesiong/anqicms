import { getCategories, pluginSaveUserGroupInfo } from '@/services';
import {
  ModalForm,
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Checkbox, Divider, Empty, Space, Spin, message } from 'antd';
import React, { useEffect, useMemo, useState } from 'react';

export type UserGroupFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  group: any;
};

type CategoryOption = {
  value: number;
  label: string;
};

type PublishCategoryPickerProps = {
  value?: number[];
  onChange?: (value: number[]) => void;
  options: CategoryOption[];
};

/**
 * 可投稿栏目：多选 + 全选。
 *
 * 全选框的勾选/半选状态完全由当前值推导，**不另存一份 allChecked 状态**：
 * 存了就一定会和真实选中集合漂移（先点全选、再取消其中一个，全选框会停在勾上）。
 * 这样「全选」和「逐个勾」写进表单的都是同一份 number[]，
 * 保存时只有一个分支，也就不会出现两套语义。
 */
const PublishCategoryPicker: React.FC<PublishCategoryPickerProps> = ({
  value,
  onChange,
  options,
}) => {
  const selected = Array.isArray(value) ? value : [];
  const allIds = options.map((item) => item.value);
  // 只认当前还存在的栏目：历史授权里可能有已被删除的栏目 ID，
  // 它们既不该被显示，也不该参与「全选是否勾满」的判断。
  const visibleSelected = allIds.filter((id) => selected.includes(id));
  const allChecked = allIds.length > 0 && visibleSelected.length === allIds.length;
  const indeterminate = visibleSelected.length > 0 && !allChecked;

  return (
    <div>
      <Checkbox
        checked={allChecked}
        indeterminate={indeterminate}
        onChange={(e) => onChange?.(e.target.checked ? allIds : [])}
      >
        <FormattedMessage id="plugin.group.publish_category.select_all" />
        <span style={{ color: '#999', marginLeft: 8 }}>
          {visibleSelected.length}/{allIds.length}
        </span>
      </Checkbox>
      <Divider style={{ margin: '8px 0' }} />
      <Checkbox.Group
        value={visibleSelected}
        onChange={(values) => onChange?.(values as number[])}
        style={{ width: '100%' }}
      >
        <Space
          direction="vertical"
          size={4}
          style={{ width: '100%', maxHeight: 240, overflowY: 'auto' }}
        >
          {options.map((item) => (
            <Checkbox key={item.value} value={item.value}>
              {item.label}
            </Checkbox>
          ))}
        </Space>
      </Checkbox.Group>
    </div>
  );
};

const UserGroupForm: React.FC<UserGroupFormProps> = (props) => {
  const intl = useIntl();
  // null 表示「还没拿到栏目列表」（加载中或加载失败），两种状态必须区别对待：
  // 加载失败时绝不能把授权当成空数组提交，那等于一次保存抹掉管理员配好的权限。
  const [categoryOptions, setCategoryOptions] = useState<CategoryOption[] | null>(
    null,
  );
  const [categoryError, setCategoryError] = useState<boolean>(false);

  useEffect(() => {
    let mounted = true;
    getCategories({ type: 1 })
      .then((res: any) => {
        if (!mounted) {
          return;
        }
        // 业务失败也是 HTTP 200，只能看 code —— 不能只判 .catch。
        if (!res || res.code !== 0) {
          setCategoryError(true);
          return;
        }
        const list = Array.isArray(res.data) ? res.data : [];
        setCategoryOptions(
          list.map((item: any) => ({
            value: item.id,
            // spacer 是后端拼好的层级前缀（如「└－」），纯文本渲染即可
            label: `${item.spacer || ''}${item.title}`,
          })),
        );
      })
      .catch(() => {
        if (mounted) {
          setCategoryError(true);
        }
      });

    return () => {
      mounted = false;
    };
  }, []);

  const initialValues = useMemo(() => {
    return { ...(props.group || {}) };
  }, [props.group]);

  const onSubmit = async (values: any) => {
    const payload: any = { ...props.group, ...values };
    if (categoryOptions === null) {
      // 栏目没加载出来：不传该字段，后端按 nil 处理 = 不改动现有授权。
      delete payload.publish_category_ids;
    } else {
      const validIds = categoryOptions.map((item) => item.value);
      payload.publish_category_ids = (values.publish_category_ids || []).filter(
        (id: number) => validIds.includes(id),
      );
    }
    const res = await pluginSaveUserGroupInfo(payload);
    message.info(res.msg);

    props.onSubmit();
  };

  return (
    <ModalForm
      width={600}
      title={
        props.group?.id
          ? intl.formatMessage({ id: 'plugin.group.edit' })
          : intl.formatMessage({ id: 'plugin.group.add' })
      }
      initialValues={initialValues}
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
      <ProFormText
        name="title"
        label={intl.formatMessage({ id: 'plugin.group.name' })}
      />
      <ProFormTextArea
        name="description"
        label={intl.formatMessage({ id: 'plugin.group.description' })}
      />
      <ProFormDigit
        name="level"
        label={intl.formatMessage({ id: 'plugin.group.level' })}
        width="sm"
        fieldProps={{
          precision: 0,
          addonAfter: intl.formatMessage({ id: 'plugin.group.level.suffix' }),
        }}
        extra={intl.formatMessage({ id: 'plugin.group.level.description' })}
      />
      <ProFormDigit
        name="price"
        label={intl.formatMessage({ id: 'plugin.group.price' })}
        width="sm"
        fieldProps={{
          precision: 0,
          addonAfter: intl.formatMessage({ id: 'plugin.group.price.suffix' }),
        }}
        extra={intl.formatMessage({ id: 'plugin.group.price.description' })}
      />
      <ProFormDigit
        name={['setting', 'expire_day']}
        label={intl.formatMessage({ id: 'plugin.group.expire_day' })}
        width="sm"
        fieldProps={{
          precision: 0,
          addonAfter: intl.formatMessage({
            id: 'plugin.group.expire_day.suffix',
          }),
        }}
        extra={intl.formatMessage({
          id: 'plugin.group.expire_day.description',
        })}
      />
      <ProFormText
        label={intl.formatMessage({ id: 'plugin.group.content_safe' })}
      >
        <Space>
          <ProFormCheckbox name={['setting', 'content_no_verify']}>
            <FormattedMessage id="plugin.group.content_safe.no-verify" />
          </ProFormCheckbox>
          <ProFormCheckbox name={['setting', 'content_no_captcha']}>
            <FormattedMessage id="plugin.group.content_safe.no-captcha" />
          </ProFormCheckbox>
        </Space>
      </ProFormText>
      <ProForm.Item
        name="publish_category_ids"
        label={intl.formatMessage({ id: 'plugin.group.publish_category' })}
        extra={intl.formatMessage({
          id: 'plugin.group.publish_category.description',
        })}
      >
        {categoryError ? (
          <Alert
            type="warning"
            showIcon
            message={intl.formatMessage({
              id: 'plugin.group.publish_category.load_failed',
            })}
          />
        ) : categoryOptions === null ? (
          <Spin
            tip={intl.formatMessage({
              id: 'plugin.group.publish_category.loading',
            })}
          >
            <div style={{ height: 80 }} />
          </Spin>
        ) : categoryOptions.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={intl.formatMessage({
              id: 'plugin.group.publish_category.empty',
            })}
          />
        ) : (
          <PublishCategoryPicker options={categoryOptions} />
        )}
      </ProForm.Item>
      <Divider>
        <FormattedMessage id="plugin.group.permission" />
      </Divider>
      <ProFormDigit
        name={['setting', 'share_reward']}
        label={intl.formatMessage({ id: 'plugin.group.share_reward' })}
        width="sm"
        fieldProps={{ precision: 0, addonAfter: '%' }}
        extra={intl.formatMessage({
          id: 'plugin.group.share_reward.description',
        })}
      />
      <ProFormDigit
        name={['setting', 'parent_reward']}
        label={intl.formatMessage({ id: 'plugin.group.parent_reward' })}
        width="sm"
        fieldProps={{ precision: 0, addonAfter: '%' }}
        extra={intl.formatMessage({
          id: 'plugin.group.parent_reward.description',
        })}
      />
      <ProFormDigit
        name={['setting', 'share_discount']}
        label={intl.formatMessage({ id: 'plugin.group.share_discount' })}
        width="sm"
        fieldProps={{ precision: 0, addonAfter: '%' }}
        extra={intl.formatMessage({
          id: 'plugin.group.share_discount.description',
        })}
      />
      <ProFormDigit
        name={['setting', 'discount']}
        label={intl.formatMessage({ id: 'plugin.group.discount' })}
        width="sm"
        fieldProps={{ precision: 0, addonAfter: '%' }}
        extra={intl.formatMessage({ id: 'plugin.group.discount.description' })}
      />
      <ProFormDigit
        name={['setting', 'shipping_discount']}
        label={intl.formatMessage({ id: 'plugin.group.shipping_discount' })}
        width="sm"
        fieldProps={{ precision: 0, addonAfter: '%' }}
        extra={intl.formatMessage({
          id: 'plugin.group.shipping_discount.description',
        })}
      />
    </ModalForm>
  );
};

export default UserGroupForm;
