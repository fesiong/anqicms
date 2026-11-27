import { getArchiveInfo } from '@/services';
import { useIntl } from '@umijs/max';
import { Button, Space, Tag } from 'antd';
import React, { useEffect, useState } from 'react';
import ArchiveSearch from './index';
import {
  loadSelectedArchives,
  mergeSelectedArchives,
  SelectedArchive,
} from './selection';

type ArchivePickerProps = {
  value?: number[];
  onChange?: (ids: number[]) => void;
  disabled?: boolean;
  id?: string;
};

const ArchivePicker: React.FC<ArchivePickerProps> = ({
  value,
  onChange,
  disabled,
  id,
}) => {
  const intl = useIntl();
  const [open, setOpen] = useState(false);
  const [archives, setArchives] = useState<SelectedArchive[]>([]);
  const ids = value || [];
  const idsKey = ids.join(',');

  useEffect(() => {
    let cancelled = false;
    const missing = ids.filter(
      (archiveId) => !archives.some((row) => row.id === archiveId),
    );
    if (missing.length) {
      loadSelectedArchives(missing, getArchiveInfo).then((rows) => {
        if (!cancelled) setArchives((previous) => previous.concat(rows));
      });
    }
    return () => {
      cancelled = true;
    };
  }, [idsKey]);

  const selected = mergeSelectedArchives(archives, ids, []);
  return (
    <Space wrap id={id}>
      {selected.map((archive) => (
        <Tag
          key={archive.id}
          closable={!disabled}
          onClose={() =>
            onChange?.(ids.filter((archiveId) => archiveId !== archive.id))
          }
        >
          {archive.title}
        </Tag>
      ))}
      <Button disabled={disabled} onClick={() => setOpen(true)}>
        {intl.formatMessage({ id: 'component.archive.select' })}
      </Button>
      {open && (
        <ArchiveSearch
          open
          selectedArchives={selected}
          onCancel={() => setOpen(false)}
          onSubmit={async (rows) => {
            setArchives(rows);
            onChange?.(rows.map((row) => row.id));
            setOpen(false);
          }}
        />
      )}
    </Space>
  );
};

export default ArchivePicker;
