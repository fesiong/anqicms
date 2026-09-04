FROM golang:alpine AS builder

# 使用阿里云 Alpine 镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add build-base

WORKDIR /build
COPY . ./
# 如果 system 目录不存在或为空（.gitignore 排除），创建占位文件以通过 go:embed
RUN if [ ! -d system ] || [ -z "$(ls -A system 2>/dev/null)" ]; then mkdir -p system && echo "placeholder" > system/placeholder.txt; fi
RUN go env -w GO111MODULE=on && go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod tidy
RUN go mod vendor
RUN rm -f anqicms.syso
RUN go build -trimpath -ldflags '-w -s' -o /build/anqicms kandaoni.com/anqicms/main

FROM alpine:latest

# 使用阿里云 Alpine 镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /app
RUN mkdir -p -v /app/cache
RUN mkdir -p -v /app/public
COPY --from=builder /build/anqicms /app/anqicms
COPY --from=builder /build/public/static /app/public/static
COPY --from=builder /build/public/*.xsl /app/public/
COPY --from=builder /build/template /app/template
COPY --from=builder /build/locales /app/locales
COPY --from=builder /build/License /app/License
COPY --from=builder /build/clientFiles /app/clientFiles
COPY --from=builder /build/dictionary.txt /app/dictionary.txt
COPY --from=builder /build/source/cwebp_linux_amd64 /app/source/
VOLUME /app/template
VOLUME /app/public
VOLUME /app/data

EXPOSE 8001
CMD ["/app/anqicms","-port", "8001"]
