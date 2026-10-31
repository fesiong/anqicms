import AttachmentSelect from '@/components/attachment';
import {
  getArchiveInfo,
  getArchives,
  pluginGetActivityDetail,
  pluginSaveActivity,
} from '@/services';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProFormDateTimePicker,
  ProFormDigit,
  ProFormFieldSet,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Card, Dropdown, Image, MenuProps, Modal, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';

export type ActivityFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  activity: any;
};

const ActivityForm: React.FC<ActivityFormProps> = (props) => {
  const [loading, setLoading] = useState(false);
  const [activity, setActivity] = useState<any>({});
  const [fetched, setFetched] = useState(false);
  const formRef = React.createRef<ProFormInstance>();
  const [activityImages, setActivityImages] = useState<any[]>([]);
  const intl = useIntl();
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'plugin.activity.goods.empty',
      }),
    },
  ]);
  const [selectedArchive, setSelectedArchive] = useState<any>({});

  const getSelectArchive = async (id: number) => {
    const res = await getArchiveInfo({
      id: id,
    });
    if (res.data) {
      setSelectedArchive(res.data);
    }
  };

  useEffect(() => {
    pluginGetActivityDetail({ id: props.activity.id }).then((res) => {
      const activity1 = res.data || {};
      activity1.price = activity1.price > 0 ? activity1.price / 100 : 0;
      activity1.start_moment =
        activity1.start_time > 0 ? dayjs(activity1.start_time * 1000) : null;
      activity1.end_moment =
        activity1.end_time > 0 ? dayjs(activity1.end_time * 1000) : null;
      activity1.skus = activity1.skus?.map((sku: any) => {
        return {
          ...sku,
          price: sku.price > 0 ? sku.price / 100 : 0,
        };
      });

      if (activity1.goods_id > 0) {
        getSelectArchive(activity1.goods_id);
      }
      setActivityImages(activity1.images || []);
      setActivity(activity1);
      setFetched(true);
    });
  }, []);
  const onSubmit = async (values: any) => {
    if (loading) return;
    setLoading(true);
    const hide = message.loading(
      {
        content: intl.formatMessage({ id: 'setting.system.submitting' }),
        key: 'loading',
      },
      0,
    );

    let postData = Object.assign({ ...activity }, values);
    postData.images = activityImages;
    postData.price = Number((postData.price * 100).toFixed(0));
    if (selectedArchive.skus) {
      if (!postData.skus) {
        postData.skus = [];
      }
      for (let i = 0; i < selectedArchive.skus.length; i++) {
        if (!postData.skus[i]) {
          postData.skus[i] = {};
        }
        postData.skus[i] = {
          sku_id: selectedArchive.skus[i].id,
          title: selectedArchive.skus[i].title,
          goods_id: selectedArchive.id,
          price: selectedArchive.skus[i].price / 100,
          stock: selectedArchive.skus[i].stock,
          ...postData.skus[i],
        };
      }
    }
    if (postData.skus) {
      postData.skus = postData.skus?.map((sku: any) => {
        return {
          ...sku,
          price: Number((sku.price * 100).toFixed(0)),
        };
      });
      if (postData.skus && postData.skus.length > 0) {
        postData.stock = postData.skus.reduce(
          (acc: number, sku: any) => acc + (sku.stock || 0),
          0,
        );
      }
    }

    await pluginSaveActivity(postData);
    hide();
    setLoading(false);
    props.onSubmit();
  };

  const handleChangeSkuValue = (index: number, field: string, value: any) => {
    // todo
    const skuValues = activity.skus || [];
    if (!skuValues[index]) {
      skuValues[index] = {};
    }
    skuValues[index][field] = value;
    setActivity({
      ...activity,
      skus: skuValues,
    });
  };

  const handleSelectImages = (rows: any) => {
    for (const row of rows) {
      let exists = false;
      for (let i in activityImages) {
        if (activityImages[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        activityImages.push(row.file_path);
      }
    }
    setActivityImages([...activityImages]);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleCleanImages = (index: number, e: any) => {
    e.stopPropagation();
    activityImages.splice(index, 1);
    setActivityImages([...activityImages]);
  };

  const onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      if (selectedArchive) {
        searchItems.push(selectedArchive);
      } else {
        searchItems.push({
          id: 0,
          title: intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        });
      }
      setSearchArchives(searchItems.concat(res.data || []));
    });
  };

  const onSkuMenuClick = (e: any) => {
    if (e.key === 'price') {
      // 批量设置价格
      let fieldValue = 0.0;
      Modal.confirm({
        icon: null,
        title: intl.formatMessage({
          id: 'content.product.batch-price',
        }),
        content: (
          <ProFormDigit
            fieldProps={{
              precision: 0,
              onChange: (value: any) => {
                fieldValue = value;
              },
              min: 1,
              max: 100,
              suffix: '%',
            }}
          />
        ),
        onOk: () => {
          // 批量设置价格
          const skus = activity.skus || [];
          for (let i = 0; i < selectedArchive.skus.length; i++) {
            if (!skus[i]) {
              skus[i] = {};
            }
            skus[i].price = (
              (fieldValue * selectedArchive.skus[i].price) /
              100 /
              100
            ).toFixed(2);
          }
          formRef?.current?.setFieldsValue({ skus });
          setActivity({
            ...activity,
            skus,
          });
        },
      });
    } else if (e.key === 'stock') {
      // 批量设置库存
      let fieldValue = 0;
      Modal.confirm({
        icon: null,
        title: intl.formatMessage({
          id: 'content.product.batch-stock',
        }),
        content: (
          <ProFormDigit
            fieldProps={{
              precision: 0,
              onChange: (value: any) => {
                fieldValue = value;
              },
            }}
          />
        ),
        onOk: () => {
          // 批量设置价格
          const skus = activity.skus || [];
          for (let i = 0; i < selectedArchive.skus.length; i++) {
            if (!skus[i]) {
              skus[i] = {};
            }
            skus[i].stock = fieldValue;
            if (skus[i].stock > selectedArchive.skus[i].stock) {
              skus[i].stock = selectedArchive.skus[i].stock;
            }
          }
          formRef?.current?.setFieldsValue({ skus });
          setActivity({
            ...activity,
            skus,
          });
        },
      });
    }
  };

  const skuMenu: MenuProps = {
    onClick: (e) => {
      onSkuMenuClick(e);
    },
    items: [
      {
        key: 'price',
        label: intl.formatMessage({
          id: 'content.product.batch-price',
        }),
      },
      {
        key: 'stock',
        label: intl.formatMessage({
          id: 'content.product.batch-stock',
        }),
      },
    ],
  };

  return (
    <>
      {fetched && (
        <ModalForm
          width={800}
          title={
            props.activity?.id
              ? intl.formatMessage({ id: 'plugin.activity.edit' })
              : intl.formatMessage({ id: 'plugin.activity.add' })
          }
          formRef={formRef}
          initialValues={activity}
          open={props.open}
          //layout="horizontal"
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
            label={intl.formatMessage({ id: 'plugin.activity.title' })}
          />
          <ProFormTextArea
            name="description"
            label={intl.formatMessage({ id: 'plugin.activity.description' })}
          />
          <ProFormFieldSet>
            <ProFormDateTimePicker
              name="start_moment"
              label={intl.formatMessage({ id: 'plugin.activity.start-time' })}
              transform={(value) => {
                return {
                  start_time: value ? dayjs(value).unix() : 0,
                };
              }}
            />
            <ProFormDateTimePicker
              name="end_moment"
              label={intl.formatMessage({ id: 'plugin.activity.end-time' })}
              transform={(value) => {
                return {
                  end_time: value ? dayjs(value).unix() : 0,
                };
              }}
            />
          </ProFormFieldSet>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.activity.goods' })}
          >
            <ProFormSelect
              name="goods_id"
              showSearch
              options={searchArchives.map((a: any) => ({
                title: a.title,
                label: a.title,
                value: a.id,
              }))}
              fieldProps={{
                onSearch: (e) => {
                  onSearchArchives(e);
                },
                onSelect: (value) => {
                  getSelectArchive(Number(value));
                },
              }}
            />
          </ProFormText>
          {selectedArchive.id && (
            <>
              <ProFormFieldSet
                label={intl.formatMessage({ id: 'plugin.activity.price' })}
              >
                <ProFormText
                  disabled
                  name="origin_price"
                  fieldProps={{
                    addonBefore: intl.formatMessage({
                      id: 'plugin.activity.origin-price',
                    }),
                    value: selectedArchive.price
                      ? (selectedArchive.price / 100).toFixed(2)
                      : 0,
                  }}
                ></ProFormText>
                <ProFormDigit
                  fieldProps={{
                    addonBefore: intl.formatMessage({
                      id: 'plugin.activity.price',
                    }),
                  }}
                  name="price"
                ></ProFormDigit>
              </ProFormFieldSet>
              <ProFormFieldSet
                label={intl.formatMessage({
                  id: 'plugin.activity.stock',
                })}
              >
                <ProFormDigit
                  disabled
                  name="origin_stock"
                  fieldProps={{
                    addonBefore: intl.formatMessage({
                      id: 'content.product.stock',
                    }),
                    value: selectedArchive.stock || 0,
                  }}
                ></ProFormDigit>
                <ProFormDigit
                  fieldProps={{
                    max: selectedArchive.stock || 0,
                    addonBefore: intl.formatMessage({
                      id: 'plugin.activity.stock',
                    }),
                  }}
                  name="stock"
                ></ProFormDigit>
                <ProFormDigit
                  fieldProps={{
                    max: selectedArchive.stock || 0,
                    addonBefore: intl.formatMessage({
                      id: 'plugin.activity.user-limit',
                    }),
                  }}
                  name="user_limit"
                ></ProFormDigit>
              </ProFormFieldSet>
            </>
          )}
          {selectedArchive.skus && (
            <Card
              bordered={false}
              className="aside-card"
              size="small"
              title={intl.formatMessage({
                id: 'content.product.option.skus',
              })}
              extra={
                <Dropdown menu={skuMenu}>
                  <a>
                    <FormattedMessage id="content.product.option.skus.extra" />
                  </a>
                </Dropdown>
              }
            >
              <div className="ant-table">
                <div className="ant-table-container">
                  <div className="ant-table-content">
                    <table className="archive-skus">
                      <colgroup>
                        <col width="30%" />
                        <col width="15%" />
                        <col width="20%" />
                        <col width="15%" />
                        <col width="20%" />
                      </colgroup>
                      <thead className="ant-table-thead">
                        <tr>
                          <th className="ant-table-cell">
                            <FormattedMessage id="content.product.option.sku.name" />
                          </th>
                          <th className="ant-table-cell">
                            <FormattedMessage id="plugin.activity.origin-price" />
                          </th>
                          <th className="ant-table-cell">
                            <FormattedMessage id="content.product.option.sku.price" />
                          </th>
                          <th className="ant-table-cell">
                            <FormattedMessage id="content.product.stock" />
                          </th>
                          <th className="ant-table-cell">
                            <FormattedMessage id="content.product.option.sku.stock" />
                          </th>
                        </tr>
                      </thead>
                      <tbody className="ant-table-tbody">
                        {selectedArchive.skus.map(
                          (item: any, index: number) => (
                            <tr className="sku-item" key={index}>
                              <td className="sku-item-value">{item.title}</td>
                              <td align="center" className="sku-item-value">
                                {(item.price / 100).toFixed(2)}
                              </td>
                              <td>
                                <ProFormDigit
                                  name={['skus', index, 'price']}
                                  fieldProps={{
                                    precision: 2,
                                    onChange: (e) => {
                                      handleChangeSkuValue(index, 'price', e);
                                    },
                                  }}
                                />
                              </td>
                              <td align="center">{item.stock}</td>
                              <td>
                                <ProFormDigit
                                  name={['skus', index, 'stock']}
                                  placeholder={'Max:' + item.stock}
                                  fieldProps={{
                                    max: item.stock,
                                    precision: 0,
                                    onChange: (e) => {
                                      handleChangeSkuValue(index, 'stock', e);
                                    },
                                  }}
                                />
                              </td>
                            </tr>
                          ),
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            </Card>
          )}

          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.coupon.status' })}
            name="status"
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.coupon.status.hide' }),
                value: 0,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.status.show' }),
                value: 1,
              },
            ]}
          />
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.activity.can-use-coupon' })}
            name="can_use_coupon"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.activity.can-use-coupon.false',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.activity.can-use-coupon.true',
                }),
                value: true,
              },
            ]}
          />
          <Card
            className="aside-card"
            size="small"
            title={intl.formatMessage({ id: 'plugin.activity.images' })}
          >
            {activityImages.length
              ? activityImages.map((item: string, index: number) => (
                  <div className="ant-upload-item" key={index}>
                    <Image
                      preview={{
                        src: item,
                      }}
                      src={item}
                    />
                    <span
                      className="delete"
                      onClick={handleCleanImages.bind(this, index)}
                    >
                      <DeleteOutlined />
                    </span>
                  </div>
                ))
              : null}
            <AttachmentSelect
              onSelect={handleSelectImages}
              open={false}
              multiple={true}
            >
              <div className="ant-upload-item">
                <div className="add">
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>
                    <FormattedMessage id="setting.system.upload" />
                  </div>
                </div>
              </div>
            </AttachmentSelect>
          </Card>
        </ModalForm>
      )}
    </>
  );
};

export default ActivityForm;
