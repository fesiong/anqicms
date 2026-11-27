import { Avatar, Image, ImageProps } from 'antd';
import React from 'react';
import VideoPlayer from '../VideoPlayer';
import './index.less';

const VIDEO_PLACEHOLDER = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 100"><rect width="160" height="100" fill="#f5f5f5"/><path d="M62 29a4 4 0 0 1 6-3l28 18a4 4 0 0 1 0 7L68 76a4 4 0 0 1-6-3z" fill="#bfbfbf"/></svg>',
)}`;

const ImageItem: React.FC<
  ImageProps & {
    isImage?: number;
    previewSrc?: string;
    posterSrc?: string;
    timestamp?: any;
    size?: any;
  }
> = (props) => {
  const isVideo = props.isImage === 2;
  const videoPoster =
    isVideo && /\.(?:jpe?g|png|gif|webp|avif)(?:[?#].*)?$/i.test(props.posterSrc || '')
      ? props.posterSrc
      : undefined;
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
  return isImage || isVideo ? (
    <Image
      width={'100%'}
      className="img"
      preview={
        props.preview === false
          ? false
          : isVideo ||
            props.src?.endsWith('.mp4') ||
            props.src?.endsWith('.webm')
          ? {
              src: props.previewSrc || props.src + '?t=' + props.timestamp,
              imageRender: () => (
                <VideoPlayer
                  src={props.previewSrc || props.src}
                  poster={videoPoster}
                />
              ),
            }
          : props.preview || {
              src: props.previewSrc || props.src + '?t=' + props.timestamp,
            }
      }
      src={isVideo ? videoPoster || VIDEO_PLACEHOLDER : props.src + '?t=' + props.timestamp}
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
