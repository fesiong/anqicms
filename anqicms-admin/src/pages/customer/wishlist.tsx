import { pluginGetWishlist } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Space } from 'antd';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

const CustomerWishlist: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.user-name' }),
      dataIndex: 'user_name',
      key: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.product.name' }),
      dataIndex: 'title',
      render: (dom: any, entity) => {
        return (
          <div>
            {dom}{' '}
            {entity.sku_title && (
              <p className="text-muted">{entity.sku_title}</p>
            )}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.thumb.name' }),
      dataIndex: 'thumb',
      hideInSearch: true,
      width: 70,
      render: (text, record) => {
        return text ? <img src={record.thumb} className="list-thumb" /> : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.origin-price' }),
      dataIndex: 'origin_price',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.wish-price' }),
      dataIndex: 'target_price',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.price.name' }),
      dataIndex: 'price',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.remark' }),
      dataIndex: 'remark',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.priority' }),
      dataIndex: 'priority',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.wishlist.created-time' }),
      dataIndex: 'created_time',
      hideInSearch: true,
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: () => <Space size={20}>-</Space>,
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'menu.customer.wishlist' })}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetWishlist(params);
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
    </PageContainer>
  );
};

export default CustomerWishlist;
