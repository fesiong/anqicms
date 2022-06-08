import {
  Pagination,
  Button,
  Space,
  Row,
  Col,
  Image,
  Modal,
  Input,
  message,
  Checkbox,
  Popover,
  Table,
  Select,
  Empty,
  Upload,
  Card,
  Avatar,
} from 'antd';
import React from 'react';
import './index.less';
import { LoadingOutlined } from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-layout';
import {
  changeAttachmentCategory,
  deleteAttachment,
  getAttachmentCategories,
  getAttachments,
  uploadAttachment,
} from '@/services/attachment';
import AttachmentCategory from './components/category';
import ProForm, { ProFormSelect } from '@ant-design/pro-form';

export default class ImageList extends React.Component {
  state = {
    images: [],
    fetched: false,
    total: 0,
    page: 1,
    limit: 18,
    selectedIds: [],
    addImageVisible: false,
    categories: [],
    categoryId: 0,
    tmpCategoryId: 0,
  };

  componentDidMount() {
    this.getImageList();
    this.getCategories();
  }

  getImageList = () => {
    const { page, limit, categoryId } = this.state;
    getAttachments({
      current: page,
      pageSize: limit,
      category_id: categoryId,
    })
      .then((res) => {
        this.setState({
          images: res.data,
          total: res.total,
          fetched: true,
        });
      })
      .catch((err) => {});
  };

  getCategories = () => {
    getAttachmentCategories().then((res) => {
      this.setState({
        categories: res.data || [],
      });
    });
  };

  handleUploadImage = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    uploadAttachment(formData).then((res) => {
      message.info({ content: res.msg, key: 'single-message' });
      this.getImageList();
    });
  };

  handleDeleteImage = () => {
    Modal.confirm({
      content: '确定要删除选中的图片吗？',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        const { selectedIds } = this.state;
        for (let id of selectedIds) {
          let res = await deleteAttachment({
            id: id,
          });
          message.info({ content: res.msg, key: 'single-message' });
        }

        this.getImageList();
      },
    });
  };

  onChangeSelect = (e: any) => {
    this.setState({
      selectedIds: e,
    });
  };

  onChangePage = (page: number, pageSize?: number) => {
    const { limit } = this.state;
    this.setState(
      {
        page: page,
        limit: pageSize ? pageSize : limit,
      },
      () => {
        this.getImageList();
      },
    );
  };

  handleChangeCategory = async (e: any) => {
    this.setState(
      {
        categoryId: e,
        page: 1,
      },
      () => {
        this.getImageList();
      },
    );
  };

  handleSetTmpCategoryId = (e: any) => {
    this.setState(
      {
        tmpCategoryId: e,
      });
  }

  handleUpdateToCategory = async (e: any) => {
    const {tmpCategoryId, categories} = this.state
    Modal.confirm({
      icon: '',
      title: '移动到新分类',
      content: <div>
        <Select
          defaultValue={tmpCategoryId}
          onChange={this.handleSetTmpCategoryId}
          style={{ width: 200 }}
        >
          <Select.Option value={0}>未分类</Select.Option>
          {categories.map((item: any, index) => (
            <Select.Option key={item.id} value={item.id}>
              {item.title}
            </Select.Option>
          ))}
        </Select>
      </div>,
      onOk: () => {
        let {selectedIds, tmpCategoryId} = this.state
        changeAttachmentCategory({
          category_id: tmpCategoryId,
          ids: selectedIds,
        }).then(res => {
          message.info(res.msg)
          this.getImageList();
        })
      }
    })
  };

  render() {
    const { images, total, limit, categories, categoryId, fetched, selectedIds } = this.state;

    return (
      <PageContainer>
        <Card
          className="image-page"
          title="图片资源管理"
          extra={
            <div className="meta">
              <Space size={16}>
                <span>分类筛选</span>
                <Select
                  defaultValue={categoryId}
                  style={{ width: 120 }}
                  onChange={this.handleChangeCategory}
                >
                  <Select.Option value={0}>全部资源</Select.Option>
                  {categories.map((item: any, index) => (
                    <Select.Option key={item.id} value={item.id}>
                      {item.title}
                    </Select.Option>
                  ))}
                </Select>
                <AttachmentCategory
                  onCancel={() => {
                    this.getCategories();
                  }}
                >
                  <Button
                    key="category"
                    onClick={() => {
                      //todo
                    }}
                  >
                    分类管理
                  </Button>
                </AttachmentCategory>
                <Upload
                  name="file"
                  showUploadList={false}
                  accept=".jpg,.jpeg,.png,.gif"
                  customRequest={this.handleUploadImage}
                >
                  <Button type="primary">上传新图片</Button>
                </Upload>
                {selectedIds.length > 0 && (
                  <>
                  <Button className="btn-gray" onClick={this.handleUpdateToCategory}>
                    移动到新分类
                  </Button>
                  <Button className="btn-gray" onClick={this.handleDeleteImage}>
                    批量删除图片
                  </Button>
                  </>
                )}
              </Space>
            </div>
          }
        >
          <div className="body">
            <Checkbox.Group onChange={this.onChangeSelect} style={{ display: 'block' }}>
              {!fetched ? (
                <Empty
                  className="empty-normal"
                  image={<LoadingOutlined style={{ fontSize: '72px' }} />}
                  description="加载中..."
                ></Empty>
              ) : total > 0 ? (
                <Row gutter={[16, 16]} className="image-list">
                  {images?.map((item: any) => (
                    <Col span={4} key={item.id}>
                      <div className="image-item">
                        <div className="inner">
                          <Checkbox className="checkbox" value={item.id} />
                          {item.thumb ? (
                            <Image
                              className="img"
                              preview={{
                                src: item.logo,
                              }}
                              src={item.thumb}
                              alt={item.file_name}
                            />
                          ) : (
                            <Avatar className="default-img" size={120}>
                              {item.file_location.substring(item.file_location.lastIndexOf('.'))}
                            </Avatar>
                          )}
                          <div className="info">
                            <div>{item.file_name}</div>
                          </div>
                        </div>
                      </div>
                    </Col>
                  ))}
                </Row>
              ) : (
                <Empty className="empty-normal" description="图片夹空空如也">
                  <Upload
                    name="file"
                    showUploadList={false}
                    accept=".jpg,.jpeg,.png,.gif"
                    customRequest={this.handleUploadImage}
                  >
                    <Button type="primary">添加图片</Button>
                  </Upload>
                </Empty>
              )}
            </Checkbox.Group>
            {total > 0 && (
              <Pagination
                defaultCurrent={1}
                defaultPageSize={limit}
                total={total}
                showSizeChanger={true}
                onChange={this.onChangePage}
                style={{ marginTop: '20px' }}
              />
            )}
          </div>
        </Card>
      </PageContainer>
    );
  }
}
