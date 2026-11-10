import { Modal } from 'antd';
import AttachmentContent from './content';
import './index.less';

const Attachment = () => {};

Attachment.show = (multiple?: boolean, intl?: any) => {
  let p = new Promise((res, rej) => {
    let selectedKeys: any[] = [];
    let close: () => void;
    const onSelect = (val: any) => {
      if (!multiple) {
        close();
        res(val);
      } else {
        selectedKeys = val;
      }
    };

    const onCancel = () => {
      close();
      rej();
    };

    const onSubmit = () => {
      if (multiple) {
        res(selectedKeys);
      }
      close();
    };

    const { destroy } = Modal.info({
      icon: null,
      maskClosable: true,
      width: 950,
      wrapClassName: 'attachment-dialog',
      content: (
        <AttachmentContent
          onSelect={onSelect}
          onCancel={() => onCancel()}
          multiple={multiple}
          style={{ marginBottom: 16 }}
          intl={intl}
        />
      ),
      okText:
        intl?.formatMessage({
          id: 'component.attachment.dialog.ok',
        }) || '确定',
      onCancel: () => {
        onCancel();
      },
      onOk: () => {
        onSubmit();
      },
    });
    close = destroy;
  });

  return p;
};
export default Attachment;
