FROM golang:1.22-alpine as builder
RUN apk add make binutils
COPY / /work
WORKDIR /work
RUN make provisioner

FROM alpine:3.19
RUN apk add lvm2 e2fsprogs e2fsprogs-extra smartmontools nvme-cli util-linux lvm2-dmeventd
COPY --from=builder /work/bin/csi-lvm-provisioner /csi-lvm-provisioner
USER root
ENTRYPOINT ["/csi-lvm-provisioner"]
