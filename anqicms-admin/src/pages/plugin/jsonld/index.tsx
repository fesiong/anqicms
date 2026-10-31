import AttachmentSelect from '@/components/attachment';
import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  getCategories,
  getModules,
  pluginGetJsonLdConfig,
  pluginSaveJsonLdConfig,
} from '@/services';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProForm,
  ProFormCheckbox,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTimePicker,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Col, message, Modal, Row, Tag } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const Pluginjsonld: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const formRef = useRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>({ module: [] });
  const [fetched, setFetched] = useState<boolean>(false);
  const [modules, setModules] = useState<any[]>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [categoryModalVisible, setCategoryModalVisible] =
    useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const organizationTypes = [
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.unselect',
      }),
      value: '',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.local-business',
      }),
      value: 'LocalBusiness',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.airline',
      }),
      value: 'Airline',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.consortium',
      }),
      value: 'Consortium',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.corporation',
      }),
      value: 'Corporation',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.educational-organization',
      }),
      value: 'EducationalOrganization',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.school',
      }),
      value: 'School',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.government-organization',
      }),
      value: 'GovernmentOrganization',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.library-system',
      }),
      value: 'LibrarySystem',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.medical-organization',
      }),
      value: 'MedicalOrganization',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.news-media-organization',
      }),
      value: 'NewsMediaOrganization',
    },
    {
      label: intl.formatMessage({ id: 'plugin.jsonld.organization-types.ngo' }),
      value: 'NGO',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.performing-group',
      }),
      value: 'PerformingGroup',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.sports-organization',
      }),
      value: 'SportsOrganization',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.organization-types.workers-union',
      }),
      value: 'WorkersUnion',
    },
  ];

  const contactTypes = [
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.general',
      }),
      value: 'general',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.customer-support',
      }),
      value: 'customer support',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.technical-support',
      }),
      value: 'technical support',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.billing-support',
      }),
      value: 'billing support',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.bill-payment',
      }),
      value: 'bill payment',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.sales',
      }),
      value: 'sales',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.reservations',
      }),
      value: 'reservations',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.credit-card-support',
      }),
      value: 'credit card support',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.emergency',
      }),
      value: 'emergency',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.baggage-tracking',
      }),
      value: 'baggage tracking',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.roadside-assistance',
      }),
      value: 'roadside assistance',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.contact-type.package-tracking',
      }),
      value: 'package tracking',
    },
  ];

  const archiveTypes = [
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.article',
      }),
      value: 'Article',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.product',
      }),
      value: 'Product',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.scholarly-article',
      }),
      value: 'ScholarlyArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.blog-posting',
      }),
      value: 'BlogPosting',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.news-article',
      }),
      value: 'NewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.analysis-news-article',
      }),
      value: 'AnalysisNewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.ask-public-news-article',
      }),
      value: 'AskPublicNewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.background-news-article',
      }),
      value: 'BackgroundNewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.opinion-news-article',
      }),
      value: 'OpinionNewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.reportage-news-article',
      }),
      value: 'ReportageNewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.review-news-article',
      }),
      value: 'ReviewNewsArticle',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.web-page',
      }),
      value: 'WebPage',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.item-page',
      }),
      value: 'ItemPage',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.recipe',
      }),
      value: 'Recipe',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.course',
      }),
      value: 'Course',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.faq-page',
      }),
      value: 'FAQPage',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.how-to',
      }),
      value: 'HowTo',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.event',
      }),
      value: 'Event',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.person',
      }),
      value: 'Person',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.schema-type.place',
      }),
      value: 'Place',
    },
  ];

  const listTypes = [
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.list-type.collection-page',
      }),
      value: 'CollectionPage',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.list-type.item-list',
      }),
      value: 'ItemList',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.jsonld.list-type.detailed-item-list',
      }),
      value: 'DetailedItemList',
    },
  ];

  const getSetting = async () => {
    const res = await pluginGetJsonLdConfig();
    let setting = res.data || {};
    // 处理 opening hours 数据，将 start_time 和 end_time 合并为 time_range
    if (setting.opening_start_time && setting.opening_end_time) {
      setting.time_range = [
        dayjs(setting.opening_start_time, 'HH:mm'),
        dayjs(setting.opening_end_time, 'HH:mm'),
      ];
    }
    setSetting(setting);
    setFetched(true);
    // modules
    getModules().then((res) => {
      setModules(res.data || []);
    });
    // categories
    getCategories({ type: 1 }).then((res) => {
      setCategories(res.data || []);
    });
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleFinishedSelectCategory = async (values: any) => {
    let categorySelected = values.category_ids || [];
    if (!setting.category) {
      setting.category = [];
    }
    categorySelected.forEach((catId: number) => {
      if (!setting.category.find((cat: any) => cat.id === catId)) {
        setting.category.push({
          id: catId,
        });
      }
    });
    setSetting({
      ...setting,
      category: [...setting.category],
    });

    setCategoryModalVisible(false);
  };

  const onChangeCategorySetting = (e: any, index: number, field: string) => {
    let value = e.target ? e.target.value : e;
    setting.category[index][field] = value;
    setSetting({
      ...setting,
      category: [...setting.category],
    });
  };

  const handleChangeField = (value: any, field: string) => {
    setSetting({
      ...setting,
      [field]: value,
    });
  };

  const handleSelectImage = (row: any, field: string) => {
    setSetting({
      ...setting,
      [field]: row.logo,
    });
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleRemoveImage = (e: any, field: string) => {
    e.stopPropagation();
    setSetting({
      ...setting,
      [field]: '',
    });
  };

  const handleDeleteCategory = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.jsonld.delete-category-confirm',
      }),
      onOk: () => {
        setting.category.splice(index, 1);
        setSetting({
          ...setting,
          category: [...setting.category],
        });
      },
    });
  };

  const onAddSocialProfile = () => {
    if (!setting.social_profiles) {
      setting.social_profiles = [];
    }
    setting.social_profiles.push('');
    formRef.current?.setFieldsValue({
      social_profiles: setting.social_profiles,
    });
    setSetting({
      ...setting,
      social_profiles: [...setting.social_profiles],
    });
  };

  const handleChangeSocialProfile = (index: number, e: any) => {
    let value = e.target ? e.target.value : e;
    setting.social_profiles[index] = value;
    formRef.current?.setFieldsValue({
      social_profiles: setting.social_profiles,
    });

    setSetting({
      ...setting,
      social_profiles: [...setting.social_profiles],
    });
  };

  const handleDeleteSocialProfile = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.jsonld.delete-social-profile-confirm',
      }),
      onOk: () => {
        setting.social_profiles.splice(index, 1);
        formRef.current?.setFieldsValue({
          social_profiles: setting.social_profiles,
        });
        setSetting({
          ...setting,
          social_profiles: [...setting.social_profiles],
        });
      },
    });
  };

  const onSubmit = async (data: any) => {
    for (let i = 0; i < data.category?.length; i++) {
      const category = data.category[i];
      if (setting.category[i]) {
        category.id = setting.category[i].id;
      }
    }
    let values = Object.assign(setting, data);
    // 处理 opening hours 数据，将 time_range 拆分为 start_time 和 end_time
    if (values.time_range && values.time_range.length === 2) {
      values.opening_start_time = values.time_range[0].format('HH:mm');
      values.opening_end_time = values.time_range[1].format('HH:mm');
      delete values.time_range;
    }
    for (let i = 0; i < modules.length; i++) {
      const module = modules[i];
      if (values.module[module.id]) {
        values.module[module.id].id = module.id;
      }
    }
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveJsonLdConfig(values)
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

  const handleCategoryOpen = () => {
    checkVip(() => {
      setCategoryModalVisible(true);
    }, intl.formatMessage({ id: 'plugin.jsonld.vip-feature' }));
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <Alert
          message={
            <div>
              <p>
                <FormattedMessage id="plugin.jsonld.tips.1" />
              </p>
              <p>
                <FormattedMessage id="plugin.jsonld.tips.2" />
              </p>
              {!isVip ? (
                <p
                  className="link"
                  onClick={() => {
                    checkVip(() => {});
                  }}
                >
                  <FormattedMessage id="plugin.jsonld.vip-tip" />
                </p>
              ) : null}
            </div>
          }
        />
        <div className="mt-normal">
          {fetched && (
            <ProForm
              formRef={formRef}
              onFinish={onSubmit}
              initialValues={setting}
            >
              <ProFormRadio.Group
                name={'open'}
                label={intl.formatMessage({ id: 'plugin.jsonld.open.name' })}
                options={[
                  {
                    label: intl.formatMessage({
                      id: 'plugin.jsonld.open.false',
                    }),
                    value: false,
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.jsonld.open.true',
                    }),
                    value: true,
                  },
                ]}
                fieldProps={{
                  onChange: (e) => handleChangeField(e.target.value, 'open'),
                }}
              />
              {setting.open && (
                <Row gutter={16} className="mb-normal">
                  <Col span={12}>
                    <Card
                      size="small"
                      title={intl.formatMessage({
                        id: 'plugin.jsonld.general',
                      })}
                    >
                      <ProFormSelect
                        name={'about_page_id'}
                        label={intl.formatMessage({
                          id: 'plugin.jsonld.about-page',
                        })}
                        request={async () => {
                          let res = await getCategories({ type: 3 });
                          const tmpData = (res.data || []).map((item: any) => ({
                            label: item.title,
                            value: item.id,
                          }));
                          return tmpData;
                        }}
                      />
                      <ProFormSelect
                        name={'contact_page_id'}
                        label={intl.formatMessage({
                          id: 'plugin.jsonld.contact-page',
                        })}
                        request={async () => {
                          let res = await getCategories({ type: 3 });
                          const tmpData = (res.data || []).map((item: any) => ({
                            label: item.title,
                            value: item.id,
                          }));
                          return tmpData;
                        }}
                      />
                      <ProFormCheckbox name="include_homepage">
                        <FormattedMessage id="plugin.jsonld.include-homepage" />
                      </ProFormCheckbox>
                      <ProFormCheckbox name="include_search">
                        <FormattedMessage id="plugin.jsonld.include-search" />
                      </ProFormCheckbox>
                      <ProFormCheckbox
                        name="include_author"
                        fieldProps={{
                          onChange: (e) =>
                            handleChangeField(
                              e.target.checked,
                              'include_author',
                            ),
                        }}
                      >
                        <FormattedMessage id="plugin.jsonld.include-author" />
                      </ProFormCheckbox>
                      {setting.include_author && (
                        <>
                          <ProFormText
                            name="author"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.author',
                            })}
                          />
                          <ProFormText
                            name="author_url"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.author-url',
                            })}
                          />
                        </>
                      )}
                      <ProFormCheckbox name="include_breadcrumb">
                        <FormattedMessage id="plugin.jsonld.include-breadcrumb" />
                      </ProFormCheckbox>
                      <ProFormCheckbox name="include_comments">
                        <FormattedMessage id="plugin.jsonld.include-comments" />
                      </ProFormCheckbox>
                      <Card
                        bordered={false}
                        className="mt-normal mb-normal"
                        title={intl.formatMessage({
                          id: 'plugin.jsonld.module',
                        })}
                        extra={
                          !isVip ? (
                            <div
                              className="link"
                              onClick={() => {
                                checkVip(() => {});
                              }}
                            >
                              <FormattedMessage id="plugin.jsonld.click-view-vip" />
                            </div>
                          ) : null
                        }
                      >
                        {modules?.map((item: any) => (
                          <div className="text-groups mt-normal" key={item.id}>
                            <div className="text-group">
                              <ProFormSelect
                                name={['module', item.id, 'id']}
                                readonly={true}
                                fieldProps={{
                                  value: item.id,
                                }}
                                options={[
                                  { label: item.title, value: item.id },
                                ]}
                              />
                            </div>
                            <div className="text-group ml-normal">
                              <div className="text-key">
                                <ProFormSelect
                                  name={['module', item.id, 'list_type']}
                                  label={intl.formatMessage({
                                    id: 'plugin.jsonld.list-type',
                                  })}
                                  options={listTypes}
                                  disabled={isVip === false}
                                />
                              </div>
                              <div className="text-value">
                                <ProFormSelect
                                  name={['module', item.id, 'schema_type']}
                                  label={intl.formatMessage({
                                    id: 'plugin.jsonld.schema-type',
                                  })}
                                  options={archiveTypes}
                                  disabled={isVip === false}
                                />
                              </div>
                            </div>
                          </div>
                        ))}
                      </Card>
                      <Card
                        bordered={false}
                        className="mt-normal mb-normal"
                        title={intl.formatMessage({
                          id: 'plugin.jsonld.category',
                        })}
                        extra={
                          <Button
                            onClick={() => {
                              handleCategoryOpen();
                            }}
                          >
                            <FormattedMessage id="plugin.jsonld.add-category" />
                          </Button>
                        }
                      >
                        <div className="mb-normal text-muted">
                          <FormattedMessage id="plugin.jsonld.category.description" />
                        </div>
                        {setting.category?.map((item: any, index: number) => (
                          <div className="text-groups mt-normal" key={item.id}>
                            <div className="text-group">
                              <ProFormSelect
                                name={['category', index, 'id']}
                                readonly={true}
                                fieldProps={{
                                  value: item.id,
                                }}
                                options={[
                                  {
                                    label: (() => {
                                      const cat = categories.find(
                                        (cat: any) => cat.id === item.id,
                                      );
                                      if (!cat) {
                                        return item.id;
                                      }
                                      return (
                                        <>
                                          {cat.parents?.length > 0 && (
                                            <span className="text-muted">
                                              {cat.parents
                                                ?.map(
                                                  (parent: any) => parent.title,
                                                )
                                                .join(' > ')}
                                              {' > '}
                                            </span>
                                          )}
                                          {cat.title}
                                        </>
                                      );
                                    })(),
                                    value: item.id,
                                  },
                                ]}
                              />
                            </div>
                            <div className="text-group ml-normal">
                              <div className="text-key">
                                <ProFormSelect
                                  name={['category', index, 'list_type']}
                                  label={intl.formatMessage({
                                    id: 'plugin.jsonld.list-type',
                                  })}
                                  options={listTypes}
                                  fieldProps={{
                                    onChange: (e) => {
                                      onChangeCategorySetting(
                                        e,
                                        index,
                                        'list_type',
                                      );
                                    },
                                  }}
                                />
                              </div>
                              <div className="text-value">
                                <ProFormSelect
                                  name={['category', index, 'schema_type']}
                                  label={intl.formatMessage({
                                    id: 'plugin.jsonld.schema-type',
                                  })}
                                  options={archiveTypes}
                                  fieldProps={{
                                    onChange: (e) => {
                                      onChangeCategorySetting(
                                        e,
                                        index,
                                        'schema_type',
                                      );
                                    },
                                  }}
                                />
                              </div>
                              <div className="text-action">
                                <Tag
                                  color="red"
                                  onClick={() => handleDeleteCategory(index)}
                                >
                                  <DeleteOutlined />
                                </Tag>
                              </div>
                            </div>
                          </div>
                        ))}
                      </Card>
                    </Card>
                    <Card
                      className="mt-normal"
                      size="small"
                      title={intl.formatMessage({
                        id: 'plugin.jsonld.default-data',
                      })}
                    >
                      <ProFormText
                        label={intl.formatMessage({
                          id: 'plugin.jsonld.default-image',
                        })}
                        width="lg"
                        extra={intl.formatMessage({
                          id: 'plugin.jsonld.default-image.tips',
                        })}
                      >
                        <AttachmentSelect
                          onSelect={(row) =>
                            handleSelectImage(row, 'default_image')
                          }
                          open={false}
                        >
                          <div className="ant-upload-item">
                            {setting.default_image ? (
                              <>
                                <img
                                  src={setting.default_image}
                                  style={{ width: '100%' }}
                                />
                                <a
                                  className="delete"
                                  onClick={(e) =>
                                    handleRemoveImage(e, 'default_image')
                                  }
                                >
                                  <FormattedMessage id="setting.system.delete" />
                                </a>
                              </>
                            ) : (
                              <div className="add">
                                <PlusOutlined />
                                <div style={{ marginTop: 8 }}>
                                  <FormattedMessage id="setting.system.upload" />
                                </div>
                              </div>
                            )}
                          </div>
                        </AttachmentSelect>
                      </ProFormText>
                      <ProFormText
                        label={intl.formatMessage({
                          id: 'plugin.jsonld.brand',
                        })}
                        width="lg"
                        name={'default_brand'}
                      />
                    </Card>
                  </Col>
                  <Col span={12}>
                    <Card
                      size="small"
                      title={intl.formatMessage({
                        id: 'plugin.jsonld.knowledge-graph',
                      })}
                    >
                      <div className="mb-normal">
                        <FormattedMessage
                          id="plugin.jsonld.knowledge-graph.description"
                          defaultMessage="知识图谱是谷歌及其服务所使用的一个知识库，用于提升其搜索引擎的结果。"
                        />
                      </div>
                      <ProFormRadio.Group
                        name={'data_type'}
                        label={intl.formatMessage({
                          id: 'plugin.jsonld.data-type',
                        })}
                        options={[
                          {
                            label: intl.formatMessage({
                              id: 'plugin.jsonld.data-type.none',
                            }),
                            value: 0,
                          },
                          {
                            label: intl.formatMessage({
                              id: 'plugin.jsonld.data-type.organization',
                            }),
                            value: 1,
                          },
                          {
                            label: intl.formatMessage({
                              id: 'plugin.jsonld.data-type.person',
                            }),
                            value: 2,
                          },
                        ]}
                        disabled={isVip === false}
                        fieldProps={{
                          onChange: (e) =>
                            handleChangeField(e.target.value, 'data_type'),
                        }}
                      />
                      {setting.data_type > 0 && (
                        <>
                          {setting.data_type === 1 && (
                            <>
                              <ProFormSelect
                                name={'organization_type'}
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.organization-type',
                                })}
                                options={organizationTypes}
                                fieldProps={{
                                  onChange: (e) =>
                                    handleChangeField(e, 'organization_type'),
                                }}
                              />
                              <ProFormText
                                name={'organization_name'}
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.organization-name',
                                })}
                              />
                              <ProFormText
                                name="organization_legal_name"
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.organization-legal-name',
                                })}
                              />
                              <ProFormText
                                name="organization_url"
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.organization-url',
                                })}
                              />
                            </>
                          )}
                          {setting.data_type === 2 && (
                            <>
                              <ProFormText
                                name="person_name"
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.person-name',
                                })}
                              />
                              <ProFormText
                                name="person_job_title"
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.person-job-title',
                                })}
                              />
                              <ProFormText
                                label={intl.formatMessage({
                                  id: 'plugin.jsonld.person-image',
                                })}
                                width="lg"
                              >
                                <AttachmentSelect
                                  onSelect={(row) =>
                                    handleSelectImage(row, 'person_image')
                                  }
                                  open={false}
                                >
                                  <div className="ant-upload-item">
                                    {setting.person_image ? (
                                      <>
                                        <img
                                          src={setting.person_image}
                                          style={{ width: '100%' }}
                                        />
                                        <a
                                          className="delete"
                                          onClick={(e) =>
                                            handleRemoveImage(e, 'person_image')
                                          }
                                        >
                                          <FormattedMessage id="setting.system.delete" />
                                        </a>
                                      </>
                                    ) : (
                                      <div className="add">
                                        <PlusOutlined />
                                        <div style={{ marginTop: 8 }}>
                                          <FormattedMessage id="setting.system.upload" />
                                        </div>
                                      </div>
                                    )}
                                  </div>
                                </AttachmentSelect>
                              </ProFormText>
                            </>
                          )}

                          <ProFormSelect
                            name="contact_type"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.contact-type',
                            })}
                            options={contactTypes}
                          />
                          <ProFormText
                            name="contact_number"
                            disabled={true}
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.contact-number',
                            })}
                            extra={intl.formatMessage({
                              id: 'plugin.jsonld.contact-number.description',
                            })}
                          />
                          <ProFormText
                            name="contact_url"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.contact-url',
                            })}
                          />
                          <ProFormText
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.logo-image',
                            })}
                            width="lg"
                            extra={intl.formatMessage({
                              id: 'plugin.jsonld.logo-image.tips',
                            })}
                          >
                            <AttachmentSelect
                              onSelect={(row) =>
                                handleSelectImage(row, 'logo_image')
                              }
                              open={false}
                            >
                              <div className="ant-upload-item">
                                {setting.logo_image ? (
                                  <>
                                    <img
                                      src={setting.logo_image}
                                      style={{ width: '100%' }}
                                    />
                                    <a
                                      className="delete"
                                      onClick={(e) =>
                                        handleRemoveImage(e, 'logo_image')
                                      }
                                    >
                                      <FormattedMessage id="setting.system.delete" />
                                    </a>
                                  </>
                                ) : (
                                  <div className="add">
                                    <PlusOutlined />
                                    <div style={{ marginTop: 8 }}>
                                      <FormattedMessage id="setting.system.upload" />
                                    </div>
                                  </div>
                                )}
                              </div>
                            </AttachmentSelect>
                          </ProFormText>
                        </>
                      )}
                      <ProFormText
                        label={intl.formatMessage({
                          id: 'plugin.jsonld.social-profile',
                        })}
                        width="lg"
                        extra={intl.formatMessage({
                          id: 'plugin.jsonld.social-profile.description',
                        })}
                      >
                        <div className="text-groups">
                          {setting.social_profiles?.map(
                            (inner: any, idx: number) => (
                              <div className="text-group" key={idx}>
                                <div className="text-value">
                                  <ProFormText
                                    name={['social_profiles', idx]}
                                    fieldProps={{
                                      onChange: (e: any) => {
                                        handleChangeSocialProfile(idx, e);
                                      },
                                    }}
                                    placeholder={intl.formatMessage({
                                      id: 'plugin.jsonld.socail-profile.placeholder',
                                    })}
                                  />
                                </div>
                                <div className="text-action">
                                  <Tag
                                    color="red"
                                    onClick={() =>
                                      handleDeleteSocialProfile(idx)
                                    }
                                  >
                                    <DeleteOutlined />
                                  </Tag>
                                </div>
                              </div>
                            ),
                          )}
                          <div className="text-group">
                            <div className="text-key">
                              <Tag
                                color="blue"
                                className="add-line"
                                onClick={() => onAddSocialProfile()}
                              >
                                {intl.formatMessage({
                                  id: 'plugin.jsonld.social-profile.add',
                                })}
                              </Tag>
                            </div>
                          </div>
                        </div>
                      </ProFormText>
                    </Card>
                    {setting.data_type === 1 &&
                      setting.organization_type === 'LocalBusiness' && (
                        <Card
                          title={intl.formatMessage({
                            id: 'plugin.jsonld.local-business',
                          })}
                          size="small"
                          className="mt-normal"
                        >
                          <ProFormCheckbox.Group
                            name="opening_day_of_week"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.opening-hours',
                            })}
                            options={[
                              {
                                value: 'Monday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.monday',
                                }),
                              },
                              {
                                value: 'Tuesday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.tuesday',
                                }),
                              },
                              {
                                value: 'Wednesday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.wednesday',
                                }),
                              },
                              {
                                value: 'Thursday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.thursday',
                                }),
                              },
                              {
                                value: 'Friday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.friday',
                                }),
                              },
                              {
                                value: 'Saturday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.saturday',
                                }),
                              },
                              {
                                value: 'Sunday',
                                label: intl.formatMessage({
                                  id: 'plugin.jsonld.day-of-week.sunday',
                                }),
                              },
                            ]}
                          />

                          <ProFormTimePicker.RangePicker
                            name="time_range"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.opening-hours.time-range',
                            })}
                            fieldProps={{
                              format: 'HH:mm',
                            }}
                          />

                          <ProFormText
                            name="price_range"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.price-range',
                            })}
                          />
                          <ProFormText
                            name="geo_latitude"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.geo-latitude',
                            })}
                          />
                          <ProFormText
                            name="geo_longitude"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.geo-longitude',
                            })}
                          />
                          <ProFormText
                            name="street_address"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.street-address',
                            })}
                          />
                          <ProFormText
                            name="address_locality"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.address-locality',
                            })}
                          />
                          <ProFormText
                            name="address_region"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.address-region',
                            })}
                          />
                          <ProFormText
                            name="postal_code"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.postal-code',
                            })}
                          />
                          <ProFormText
                            name="address_country"
                            label={intl.formatMessage({
                              id: 'plugin.jsonld.address-country',
                            })}
                          />
                        </Card>
                      )}
                  </Col>
                </Row>
              )}
            </ProForm>
          )}
        </div>
      </Card>
      {categoryModalVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'content.category.select' })}
          open={categoryModalVisible}
          onOpenChange={(flag) => setCategoryModalVisible(flag)}
          onAbort={() => setCategoryModalVisible(false)}
          onFinish={(values) => handleFinishedSelectCategory(values)}
        >
          <ProFormSelect
            name="category_ids"
            mode="multiple"
            required
            options={[
              {
                title: intl.formatMessage({
                  id: 'content.please-select',
                }),
                value: 0,
                status: 1,
              },
            ]
              .concat(categories)
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
        </ModalForm>
      )}
      <VipModal />
    </NewContainer>
  );
};

export default Pluginjsonld;
