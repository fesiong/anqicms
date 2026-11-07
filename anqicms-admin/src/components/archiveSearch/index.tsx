import { getArchives, getCategories, getModules } from '@/services';
import {
  ProColumns,
  ProFormInstance,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import React, { useRef, useState } from 'react';
import { mergeSelectedArchives, SelectedArchive } from './selection';

export type ArchiveSearchProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: any[]) => Promise<void>;
  open: boolean;
  selectedArchives?: SelectedArchive[];
  disabledIds?: number[];
};

const ArchiveSearch: React.FC<ArchiveSearchProps> = (props) => {
  const [selectedRows, setSelectedRows] = useState<SelectedArchive[]>(
    props.selectedArchives || [],
  );
  const [moduleId, setModuleId] = useState<number>();
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<ProFormInstance>();
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    { title: 'ID', dataIndex: 'id', hideInSearch: true, width: 90 },
    {
      title: intl.formatMessage({ id: 'content.module.name' }),
      dataIndex: 'module_id',
      valueType: 'select',
      order: 3,
      render: (_, entity) => entity.module_name,
      request: async () => {
        const res = await getModules();
        return (res.data || []).map((item: any) => ({
          label: item.name,
          value: item.id,
        }));
      },
      fieldProps: {
        onChange: (id: number | undefined) => {
          setModuleId(id);
          formRef.current?.setFieldsValue({ category_id: undefined });
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'content.category.name' }),
      dataIndex: 'category_id',
      valueType: 'select',
      order: 2,
      params: { module_id: moduleId },
      request: async (params) => {
        const res = await getCategories({
          type: 1,
          show_type: 1,
          module_id: params.module_id,
        });
        return (res.data || []).map((cat: any) => ({
          label: [
            ...(cat.parents || []).map((parent: any) => parent.title),
            cat.title,
          ].join(' > '),
          value: cat.id,
        }));
      },
      fieldProps: {
        showSearch: true,
        optionFilterProp: 'label',
      },
      render: (_, entity) => entity.category_titles?.join(' / '),
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      order: 1,
      render: (dom, entity) => (
        <a href={entity.link} target="_blank" rel="noopener noreferrer">
          {dom}
        </a>
      ),
    },
  ];

  return (
    <Modal
      width={1100}
      title={intl.formatMessage({ id: 'component.archive.select' })}
      open={props.open}
      confirmLoading={submitting}
      onCancel={() => props.onCancel()}
      onOk={async () => {
        setSubmitting(true);
        try {
          await props.onSubmit(selectedRows);
        } finally {
          setSubmitting(false);
        }
      }}
    >
      <ProTable<any>
        formRef={formRef}
        rowKey="id"
        search={{ span: 8, defaultCollapsed: false }}
        toolBarRender={false}
        onReset={() => setModuleId(undefined)}
        tableAlertOptionRender={false}
        request={async (params) => {
          const res = await getArchives({ ...params, status: 'ok' });
          return { ...res, data: res.data || [], success: res.code === 0 };
        }}
        columns={columns}
        rowSelection={{
          preserveSelectedRowKeys: true,
          selectedRowKeys: selectedRows.map((row) => row.id),
          getCheckboxProps: (row) => ({
            disabled: props.disabledIds?.includes(row.id),
          }),
          onChange: (keys, rows) => {
            setSelectedRows((previous) =>
              mergeSelectedArchives(previous, keys as number[], rows),
            );
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
