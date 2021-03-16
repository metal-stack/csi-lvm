FROM golang:1.16-alpine as builder
RUN apk add make binutils
COPY / /work
WORKDIR /work
RUN make controller

FROM alpine:3.13
COPY --from=builder /work/bin/csi-lvm-controller /csi-lvm-controller
USER 65534
ENTRYPOINT ["/csi-lvm-controller"]
