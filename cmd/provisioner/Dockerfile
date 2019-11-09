FROM golang:1.13-alpine as builder
RUN apk add make binutils
COPY / /work
WORKDIR /work
RUN make provisioner

FROM alpine:3.10
RUN apk add lvm2 e2fsprogs util-linux
COPY --from=builder /work/bin/csi-lvm-provisioner /csi-lvm-provisioner
# default lvm config uses lvmetad and throwing below warning for all lvm tools
# WARNING: Failed to connect to lvmetad. Falling back to device scanning.
# So, ask lvm not to use lvmetad
RUN mkdir -p /etc/lvm
RUN echo "global { use_lvmetad = 0 }" >> /etc/lvm/lvm.conf && \
    echo "activation { udev_sync = 0 udev_rules = 0 }" >> /etc/lvm/lvm.conf

USER root
ENTRYPOINT ["/csi-lvm-provisioner"]
