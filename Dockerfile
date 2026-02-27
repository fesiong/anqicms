FROM golang:alpine AS builder

RUN apk add build-base

WORKDIR /build
COPY . ./
RUN go env -w GO111MODULE=on && go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod tidy
RUN go mod vendor
RUN rm -f anqicms.syso
RUN go build -trimpath -ldflags '-w -s' -o /build/anqicms kandaoni.com/anqicms/main

FROM alpine:latest

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
