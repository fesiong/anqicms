import { getPlaces, savePlace } from '@/services/place';
import {
  ModalForm,
  ProFormInstance,
  ProFormSelect,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

export type MultiPlaceAddProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  place: any;
};

const MultiPlaceAdd: React.FC<MultiPlaceAddProps> = (props) => {
  const formRef = React.createRef<ProFormInstance>();
  const [places, setPlaces] = useState<any[]>([]);
  const intl = useIntl();

  useEffect(() => {
    getPlaces().then((res) => {
      setPlaces(res.data || []);
    });
  }, []);

  const onSubmit = async (values: any) => {
    let placeData = {
      parent_id: values.parent_id,
      title: '',
      url_token: '',
      status: 1,
    };
    let splitData = values.inputs.split('\n');
    for (let item of splitData) {
      let names = item.trim().split('|');
      placeData.title = names[0];
      placeData.url_token = '';
      if (names.length > 1) {
        placeData.url_token = names[1];
      }
      if (placeData.title.length > 0) {
        let res = await savePlace(placeData);
        message.info(res.msg);
      }
    }
    props.onSubmit();
  };

  return (
    <ModalForm
      formRef={formRef}
      width={600}
      title={intl.formatMessage({ id: 'content.place.batch-add' })}
      initialValues={props.place}
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
        label={intl.formatMessage({ id: 'content.category.parent' })}
        name="parent_id"
        width="lg"
        options={[
          {
            id: 0,
            title: intl.formatMessage({
              id: 'content.place.top',
            }),
            status: 1,
          },
        ]
          .concat(
            places.filter((item) =>
              props.place?.id > 0
                ? item.id !== props.place.id &&
                  item.parent_id !== props.place.id
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
      />
      <ProFormTextArea
        name="inputs"
        width="lg"
        label={intl.formatMessage({ id: 'content.place.title' })}
        placeholder={intl.formatMessage({
          id: 'content.place.batch-name.placeholder',
        })}
        fieldProps={{
          rows: 10,
        }}
        extra={
          <div
            dangerouslySetInnerHTML={{
              __html: intl.formatMessage({
                id: 'content.place.batch-name.description',
              }),
            }}
          ></div>
        }
      />
    </ModalForm>
  );
};

export default MultiPlaceAdd;
