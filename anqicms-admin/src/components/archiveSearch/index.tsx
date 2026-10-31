import { getArchives, getCategories } from '@/services';
import { ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import React, { useState } from 'react';

export type ArchiveSearchProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: string[]) => Promise<void>;
  open: boolean;
};

const ArchiveSearch: React.FC<ArchiveSearchProps> = (props) => {
  const [selectedRows, setSelectedRows] = useState<any[]>([]);
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      render: (dom, entity) => {
        return (
          <div style={{ maxWidth: 400 }}>
            <a href={entity.link} target="_blank" rel="noopener noreferrer">
              {dom}
            </a>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.category.name' }),
      dataIndex: 'category_id',
      valueType: 'select',
      render: (_: any, entity) => {
        return (
          <div>
            {entity.category_titles?.map((item: string) => (
              <div key={item}>{item}</div>
            ))}
          </div>
        );
      },
      request: async () => {
        let res = await getCategories({ type: 1 });
        const categories = [
          {
            parents: [],
            title: intl.formatMessage({ id: 'content.category.all' }),
            id: 0,
            status: 1,
          },
        ]
          .concat(res.data || [])
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
          }));
        return categories;
      },
      fieldProps: {
        showSearch: true,
        filterOption: (input: string, option: any) =>
          (option?.title ?? option?.label)
            .toLowerCase()
            .includes(input.toLowerCase()),
      },
    },
  ];

  return (
    <Modal
      width={1000}
      title={intl.formatMessage({ id: 'component.archive.select' })}
      open={props.open}
      onCancel={() => {
        props.onCancel();
      }}
      onOk={() => {
        props.onSubmit(selectedRows);
      }}
    >
      <ProTable<any>
        rowKey="id"
        search={{
          span: { xs: 24, sm: 8, md: 8, lg: 8, xl: 8, xxl: 8 },
          defaultCollapsed: false,
        }}
        toolBarRender={false}
        tableAlertRender={false}
        tableAlertOptionRender={false}
        request={(params) => {
          params.status = 'ok';
          return getArchives(params);
        }}
        columnsState={{
          persistenceKey: 'archive-search-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (_, selectedRows) => {
            setSelectedRows(selectedRows);
          },
        }}
        pagination={{
          defaultPageSize: 10,
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
    </Modal>
  );
};

export default ArchiveSearch;
