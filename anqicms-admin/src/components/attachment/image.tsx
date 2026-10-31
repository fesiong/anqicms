import { useIntl } from '@umijs/max';
import { Avatar, Image, ImageProps } from 'antd';
import React from 'react';
import './index.less';

const ImageItem: React.FC<
  ImageProps & {
    isImage?: number;
    previewSrc?: string;
    timestamp?: any;
    size?: any;
  }
> = (props) => {
  const intl = useIntl();
  const isImage =
    props.isImage === 1 ||
    props.src?.endsWith('.png') ||
    props.src?.endsWith('.jpg') ||
    props.src?.endsWith('.jpeg') ||
    props.src?.endsWith('.gif') ||
    props.src?.endsWith('.svg') ||
    props.src?.endsWith('.webp') ||
    props.src?.endsWith('.ico') ||
    props.src?.endsWith('.bmp') ||
    props.src?.endsWith('.tiff') ||
    props.src?.endsWith('.tif');
  return isImage ? (
    <Image
      width={'100%'}
      className="img"
      preview={
        props.preview === false
          ? false
          : props.isImage === 2 ||
            props.src?.endsWith('.mp4') ||
            props.src?.endsWith('.webm')
          ? {
              src: props.previewSrc || props.src + '?t=' + props.timestamp,
              imageRender: () => (
                <video
                  controls
                  style={{ maxWidth: '100%', maxHeight: '100%' }}
                  poster={props.src}
                >
                  <source
                    src={
                      props.previewSrc || props.src + '?t=' + props.timestamp
                    }
                  />
                  {intl.formatMessage({
                    id: 'component.attachment.video-unsupport',
                  })}
                </video>
              ),
            }
          : props.preview || {
              src: props.previewSrc || props.src + '?t=' + props.timestamp,
            }
      }
      src={props.src + '?t=' + props.timestamp}
      alt={props.alt}
    />
  ) : (
    <a href={props.src} target="_blank" rel="noreferrer">
      <Avatar className="default-img" size={props.size || 200} alt={props.alt}>
        {props.src?.substring(props.src?.lastIndexOf('.')).toUpperCase()}
      </Avatar>
    </a>
  );
};

export default ImageItem;
