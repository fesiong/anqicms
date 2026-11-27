import React, { useEffect, useRef } from 'react';

const VideoPlayer: React.FC<{
  src: string;
  poster?: string;
  onError?: () => void;
}> = ({ src, poster, onError }) => {
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;
    let hls: { destroy: () => void } | undefined;
    let disposed = false;
    video.removeAttribute('src');
    video.load();

    const useNativePlayback = () => {
      video.src = src;
      video.load();
    };

    if (!src.split(/[?#]/)[0].toLowerCase().endsWith('.m3u8')) {
      useNativePlayback();
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      useNativePlayback();
    } else {
      import('hls.js')
        .then(({ default: Hls }) => {
          if (disposed) return;
          if (!Hls.isSupported()) {
            useNativePlayback();
            return;
          }
          hls = new Hls({ enableWorker: true });
          hls.loadSource(src);
          hls.attachMedia(video);
        })
        .catch(() => {
          if (!disposed) useNativePlayback();
        });
    }

    return () => {
      disposed = true;
      hls?.destroy();
      video.pause();
      video.removeAttribute('src');
      video.load();
    };
  }, [src]);

  return (
    <video
      ref={videoRef}
      controls
      playsInline
      preload="metadata"
      poster={poster || undefined}
      onError={onError}
      style={{ width: '100%', maxHeight: '70vh', background: '#000' }}
    />
  );
};

export default VideoPlayer;
