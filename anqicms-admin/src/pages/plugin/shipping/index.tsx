import NewContainer from '@/components/NewContainer';
import PlaceCascader from '@/components/PlaceCascader';
import {
  pluginGetShippingSetting,
  pluginSaveShippingSetting,
} from '@/services';
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormDigit,
  ProFormList,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Button, Card, Col, Divider, Row, Space, Tooltip, message } from 'antd';
import React, { useEffect, useState } from 'react';

interface PlaceItem {
  id?: number;
  country_code: string;
  province_code?: string;
  city_code?: string;
  country: string;
  province?: string;
  city?: string;
  address?: string; // 详细地址
  latitude?: string;
  longitude?: string;
}

interface RegionItem {
  id?: number;
  regions?: PlaceItem[];
  first_unit: number;
  first_fee: number;
  continue_unit: number;
  continue_fee: number;
}

interface ShippingRule {
  units: any[]; // 计量单位列表
  volumetric_divisor: number; // 体积单位换算比例，如 cm³/5000 = 1kg，5000 = 1kg
  weight_units: any[]; // 重量单位列表
  type: number; // 1 = 固定运费， 2 = 按重量计费， 3 = 按体积计费， 4 = 按数量计费， 5 = 按金额计费
  is_free_shipping: boolean; // 是否免费配送 false=自定义运费 true=包邮, 如果设置了包邮，则下面的配置将被忽略。
  delivery_place: PlaceItem; // 发货地址
  default_rule: RegionItem; // 默认运费规则
  region_rules: RegionItem[]; // 自定义地区运费规则
  free_shipping_rule: {
    // 满额免运费规则
    type: number; // 0=不启用，1=金额满n包邮，2=重量数满n包邮，3=体积数满n包邮，4=数量满n件包邮
    threshold: number;
  };
}

const ruleTypeOptions = [
  { label: '固定运费', value: 1 },
  { label: '按重量计费', value: 2 },
  { label: '按体积计费', value: 3 },
  { label: '按数量计费', value: 4 },
  { label: '按金额计费', value: 5 },
];

const freeShippingTypeOptions = [
  { label: '不启用', value: 0 },
  { label: '金额满x元包邮', value: 1 },
  { label: '重量满kg包邮', value: 2 },
  { label: '体积满m³包邮', value: 3 },
  { label: '数量满件包邮', value: 4 },
];

const PluginShipping: React.FC = () => {
  const [setting, setSetting] = useState<ShippingRule | null>(null);
  const [fetched, setFetched] = useState<boolean>(false);
  const [isFreeShipping, setIsFreeShipping] = useState<boolean>(false);
  const [currentType, setCurrentType] = useState<any>(1);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    setFetched(false);
    const res = await pluginGetShippingSetting();
    let settingData = res.data || {};
    // 转换分成元
    if (settingData.default_rule?.first_fee > 0) {
      settingData.default_rule.first_fee =
        settingData.default_rule?.first_fee / 100;
    }
    if (settingData.default_rule?.continue_fee > 0) {
      settingData.default_rule.continue_fee =
        settingData.default_rule?.continue_fee / 100;
    }
    settingData.region_rules = settingData.region_rules?.map((r: any) => {
      if (r.first_fee > 0) {
        r.first_fee = r.first_fee / 100;
      }
      if (r.continue_fee > 0) {
        r.continue_fee = r.continue_fee / 100;
      }
      return r;
    });
    if (!settingData.type) {
      settingData.type = 1;
    }
    setSetting(settingData);
    setCurrentType(settingData.type);
    setIsFreeShipping(settingData.is_free_shipping || false);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const getTypeNumName = () => {
    let ruleType: number = currentType || 1;

    if (ruleType === 2) {
      return 'kg';
    } else if (ruleType === 3) {
      return 'm³';
    } else if (ruleType === 4) {
      return '件';
    } else if (ruleType === 5) {
      return '元';
    }

    return '件';
  };

  const onSubmit = async (values: any) => {
    const postData = {
      ...setting,
      ...values,
      // 金额单位转换成分
      default_rule: {
        first_unit: Number(values.default_rule?.first_unit) || 1,
        first_fee:
          Number((values.default_rule?.first_fee * 100).toFixed(0)) || 0,
        continue_unit: Number(values.default_rule?.continue_unit) || 1,
        continue_fee:
          Number((values.default_rule?.continue_fee * 100).toFixed(0)) || 0,
      },
      region_rules: (values.region_rules || []).map((r: any) => ({
        regions: r.regions,
        first_unit: Number(r.first_unit) || 1,
        first_fee: Number((r.first_fee * 100).toFixed(0)) || 0,
        continue_unit: Number(r.continue_unit) || 1,
        continue_fee: Number((r.continue_fee * 100).toFixed(0)) || 0,
      })),
      free_shipping_rule: {
        type: Number(values.free_shipping_rule?.type) || 1,
        threshold: Number(values.free_shipping_rule?.threshold) || 0,
      },
    };
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveShippingSetting(postData)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const typeNumName = getTypeNumName();

  return (
    <NewContainer
      onTabChange={(key) => onTabChange(key)}
      title={intl.formatMessage({ id: 'plugin.shipping.setting' })}
    >
      <Card key={newKey}>
        {fetched && (
          <ProForm initialValues={setting} onFinish={onSubmit}>
            <Divider>计量单位设置</Divider>
            <Row gutter={16}>
              <Col span={12}>
                <ProFormList
                  name="units"
                  label={intl.formatMessage({ id: 'setting.shipping.units' })}
                  min={1}
                  actionRender={(items: any) => (
                    <Button
                      type="dashed"
                      icon={<PlusOutlined />}
                      onClick={() => {
                        items.push({ name: '', volume_name: '', ratio: 1 });
                      }}
                    >
                      {intl.formatMessage({ id: 'setting.shipping.add-unit' })}
                    </Button>
                  )}
                  copyIconProps={false}
                  deleteIconProps={false}
                >
                  {(field, index, action) => (
                    <div
                      style={{
                        display: 'flex',
                        gap: '16px',
                        alignItems: 'center',
                        marginBottom: '12px',
                        padding: '12px',
                        border: '1px dashed #d9d9d9',
                        borderRadius: '6px',
                      }}
                    >
                      <div style={{ width: '140px' }}>
                        <ProFormText
                          fieldProps={{
                            placeholder: intl.formatMessage({
                              id: 'setting.shipping.length-unit-name',
                            }),
                          }}
                          {...field}
                          name={['name']}
                          label={intl.formatMessage({
                            id: 'setting.shipping.length-unit-name',
                          })}
                          width="md"
                        />
                      </div>
                      <div style={{ width: '160px' }}>
                        <ProFormText
                          fieldProps={{
                            placeholder: intl.formatMessage({
                              id: 'setting.shipping.volume-unit-name',
                            }),
                          }}
                          {...field}
                          name={['volume_name']}
                          label={intl.formatMessage({
                            id: 'setting.shipping.volume-unit-name',
                          })}
                          width="md"
                        />
                      </div>
                      <div style={{ width: '180px' }}>
                        <ProFormDigit
                          fieldProps={{
                            min: 0,
                            step: 0.0001,
                            precision: 6,
                            placeholder: '0.01',
                            addonAfter: intl.formatMessage({
                              id: 'setting.shipping.ratio-to-m.suffix',
                            }),
                          }}
                          {...field}
                          name={['ratio']}
                          label={intl.formatMessage({
                            id: 'setting.shipping.ratio-to-m',
                          })}
                          width="md"
                        />
                      </div>
                      <div style={{ width: '80px', textAlign: 'right' }}>
                        <Tooltip
                          title={intl.formatMessage({
                            id: 'setting.system.delete',
                          })}
                        >
                          <Button
                            type="link"
                            danger
                            icon={
                              <MinusCircleOutlined
                                style={{ fontSize: '18px' }}
                              />
                            }
                            onClick={() => {
                              action.remove(index);
                            }}
                          />
                        </Tooltip>
                      </div>
                    </div>
                  )}
                </ProFormList>
              </Col>
              <Col span={12}>
                <ProFormList
                  name="weight_units"
                  label={intl.formatMessage({
                    id: 'setting.shipping.weight-units',
                  })}
                  min={1}
                  actionRender={(items: any) => (
                    <Button
                      type="dashed"
                      icon={<PlusOutlined />}
                      onClick={() => {
                        items.push({ name: '', ratio: 1 });
                      }}
                    >
                      {intl.formatMessage({ id: 'setting.shipping.add-unit' })}
                    </Button>
                  )}
                  copyIconProps={false}
                  deleteIconProps={false}
                >
                  {(field, index, action) => (
                    <div
                      style={{
                        display: 'flex',
                        gap: '16px',
                        alignItems: 'center',
                        marginBottom: '12px',
                        padding: '12px',
                        border: '1px dashed #d9d9d9',
                        borderRadius: '6px',
                      }}
                    >
                      <div style={{ width: '140px' }}>
                        <ProFormText
                          fieldProps={{
                            placeholder: intl.formatMessage({
                              id: 'setting.shipping.length-unit-name',
                            }),
                          }}
                          {...field}
                          name={['name']}
                          label={intl.formatMessage({
                            id: 'setting.shipping.weight-unit-name',
                          })}
                          width="md"
                        />
                      </div>
                      <div style={{ width: '180px' }}>
                        <ProFormDigit
                          fieldProps={{
                            min: 0,
                            step: 0.0001,
                            precision: 6,
                            placeholder: '0.01',
                            addonAfter: intl.formatMessage({
                              id: 'setting.shipping.ratio-to-kg.suffix',
                            }),
                          }}
                          {...field}
                          name={['ratio']}
                          label={intl.formatMessage({
                            id: 'setting.shipping.ratio-to-kg',
                          })}
                          width="md"
                        />
                      </div>
                      <div style={{ width: '80px', textAlign: 'right' }}>
                        <Tooltip
                          title={intl.formatMessage({
                            id: 'setting.system.delete',
                          })}
                        >
                          <Button
                            type="link"
                            danger
                            icon={
                              <MinusCircleOutlined
                                style={{ fontSize: '18px' }}
                              />
                            }
                            onClick={() => {
                              action.remove(index);
                            }}
                          />
                        </Tooltip>
                      </div>
                    </div>
                  )}
                </ProFormList>
              </Col>
            </Row>
            <ProFormDigit
              name="volumetric_divisor"
              label={intl.formatMessage({
                id: 'setting.shipping.volumetric-divisor',
              })}
              width="lg"
              fieldProps={{
                min: 0,
                step: 1,
                precision: 0,
              }}
              placeholder={intl.formatMessage({
                id: 'setting.shipping.volumetric-divisor-placeholder',
              })}
              extra={intl.formatMessage({
                id: 'setting.shipping.volumetric-divisor-description',
              })}
            />
            <Divider>基本设置</Divider>
            <ProFormRadio.Group
              name="is_free_shipping"
              label="是否包邮"
              options={[
                { label: '自定义运费', value: false },
                { label: '包邮', value: true },
              ]}
              fieldProps={{
                optionType: 'button',
                onChange: (e) => {
                  setIsFreeShipping(e.target.value);
                },
              }}
            />
            {!isFreeShipping && (
              <>
                <PlaceCascader
                  label="发货地"
                  name={'delivery_place'}
                  multiple={false}
                />

                <ProFormSelect
                  name="type"
                  label="规则类型"
                  options={ruleTypeOptions}
                  rules={[{ required: true, message: '请选择规则类型' }]}
                  fieldProps={{
                    onChange: (e) => {
                      setCurrentType(e);
                    },
                  }}
                />
                <Divider>运费计算方式</Divider>
                {/* 计费规则 */}
                <Card
                  title="计费规则"
                  size="small"
                  style={{ marginBottom: 16 }}
                >
                  <ProForm.Item label="默认地区运费">
                    {currentType === 1 ? (
                      <ProFormDigit
                        name={['default_rule', 'first_fee']}
                        fieldProps={{
                          precision: 2,
                          min: 0,
                          style: { width: 80 },
                        }}
                        initialValue={0}
                        addonAfter={<span>元</span>}
                      />
                    ) : (
                      <Space align="center">
                        <ProFormDigit
                          name={['default_rule', 'first_unit']}
                          fieldProps={{
                            precision: 0,
                            min: 1,
                            style: { width: 80 },
                          }}
                          initialValue={1}
                          addonAfter={<span>{typeNumName}内</span>}
                        />
                        <ProFormDigit
                          name={['default_rule', 'first_fee']}
                          fieldProps={{
                            precision: 2,
                            min: 0,
                            style: { width: 80 },
                          }}
                          initialValue={0}
                          addonAfter={<span>元，每增加</span>}
                        />
                        <ProFormDigit
                          name={['default_rule', 'continue_unit']}
                          fieldProps={{
                            precision: 0,
                            min: 1,
                            style: { width: 80 },
                          }}
                          initialValue={1}
                          addonAfter={<span>{typeNumName}，增加</span>}
                        />
                        <ProFormDigit
                          name={['default_rule', 'continue_fee']}
                          fieldProps={{
                            precision: 2,
                            min: 0,
                            style: { width: 80 },
                          }}
                          initialValue={0}
                          addonAfter={<span>元</span>}
                        />
                      </Space>
                    )}
                  </ProForm.Item>

                  {/* 指定地区规则 */}
                  <ProFormList
                    name="region_rules"
                    label="指定地区规则"
                    min={0}
                    creatorButtonProps={{
                      creatorButtonText: '添加指定地区',
                    }}
                    itemRender={(dom, listMeta) => (
                      <Card size="small" style={{ marginBottom: 8 }}>
                        {dom.listDom}
                        <div style={{ textAlign: 'right' }}>
                          <a
                            onClick={() =>
                              listMeta.operation.remove(listMeta.index)
                            }
                          >
                            删除
                          </a>
                        </div>
                      </Card>
                    )}
                  >
                    <PlaceCascader
                      label="地区"
                      name={'regions'}
                      multiple={true}
                    />

                    {currentType === 1 ? (
                      <ProFormDigit
                        name={'first_fee'}
                        fieldProps={{
                          precision: 2,
                          min: 0,
                          style: { width: 70 },
                        }}
                        initialValue={0}
                        addonAfter={<span>元</span>}
                      />
                    ) : (
                      <div
                        style={{
                          display: 'flex',
                          gap: 12,
                          marginTop: 8,
                          flexWrap: 'wrap',
                        }}
                      >
                        <ProFormDigit
                          name={'first_unit'}
                          fieldProps={{
                            precision: 0,
                            min: 1,
                            style: { width: 70 },
                          }}
                          initialValue={1}
                          addonAfter={<span>{typeNumName}内</span>}
                        />
                        <ProFormDigit
                          name={'first_fee'}
                          fieldProps={{
                            precision: 2,
                            min: 0,
                            style: { width: 70 },
                          }}
                          initialValue={0}
                          addonAfter={<span>元，每增加</span>}
                        />
                        <ProFormDigit
                          name={'continue_unit'}
                          fieldProps={{
                            precision: 0,
                            min: 1,
                            style: { width: 70 },
                          }}
                          initialValue={1}
                          addonAfter={<span>{typeNumName}，增加</span>}
                        />
                        <ProFormDigit
                          name={'continue_fee'}
                          fieldProps={{
                            precision: 2,
                            min: 0,
                            style: { width: 70 },
                          }}
                          initialValue={0}
                          addonAfter={<span>元</span>}
                        />
                      </div>
                    )}
                  </ProFormList>
                </Card>
                <Divider>包邮规则</Divider>
                {/* 包邮规则 */}
                <ProForm.Item label="包邮规则" tooltip="设置满额包邮条件">
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <ProFormRadio.Group
                      name={['free_shipping_rule', 'type']}
                      label="包邮类型"
                      options={freeShippingTypeOptions}
                      initialValue={0}
                    />
                    <Space>
                      <ProFormDigit
                        name={['free_shipping_rule', 'threshold']}
                        fieldProps={{
                          precision: 2,
                          min: 0,
                          style: { width: 120 },
                        }}
                        placeholder="请输入门槛金额/数量"
                        addonBefore="满"
                        addonAfter="包邮"
                      />
                    </Space>
                  </Space>
                </ProForm.Item>
              </>
            )}
          </ProForm>
        )}
      </Card>
    </NewContainer>
  );
};

export default PluginShipping;
