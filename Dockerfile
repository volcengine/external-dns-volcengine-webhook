#--------
# builder
#--------
FROM golang:1.23.6-alpine@sha256:2c49857f2295e89b23b28386e57e018a86620a8fede5003900f2d138ba9c4037 AS builder

ARG TARGETPLATFORM
ARG TARGETOS="linux"
ARG TARGETARCH="amd64" 
ARG TARGETVARIANT=""
RUN go env -w GOPROXY="https://goproxy.cn|direct"
RUN go env -w GOPRIVATE="*.everphoto.cn,git.smartisan.com"
RUN go env -w GOSUMDB="sum.golang.google.cn"    
WORKDIR /app
    
COPY go.mod go.sum /app/
    
RUN go mod download
    
COPY . .
    
RUN GOARM=$(if [ -n "${TARGETVARIANT}" ]; then echo "${TARGETVARIANT#\"v\"}"; else echo "0"; fi) && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${GOARM} \
    go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o ./external-dns-volcengine-webhook .

#--------
# container
#--------
FROM alpine:3.18

USER 20000:20000

COPY --from=builder --chmod=555 /app/external-dns-volcengine-webhook /opt/external-dns-volcengine-webhook
# ENTRYPOINT ["/opt/external-dns-volcengine-webhook"]
