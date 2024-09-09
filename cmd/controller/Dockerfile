FROM golang:1.23-alpine as builder
RUN apk add make binutils
COPY / /work
WORKDIR /work
RUN make controller

FROM alpine:3.20
COPY --from=builder /work/bin/csi-lvm-controller /csi-lvm-controller
USER 65534
ENTRYPOINT ["/csi-lvm-controller"]
