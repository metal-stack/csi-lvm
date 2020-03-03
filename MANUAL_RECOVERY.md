# Manual Recovery

In case, a machine with not the most recent version ( < v0.5.0 ) of csi-lvm was installed but rebooted, the mountpoints can be recovered manually with the following procedure:
Log into the machine and do the following steps:

1. Re-enable all logical volumes

```bash
docker run -it --rm -v /dev:/dev -v /lib/modules:/lib/modules --entrypoint /bin/sh metalstack/csi-lvm-provisioner:latest
# scan all disks for volume groups
vgscan
# activate all volume groups and their logical volumes
vgchange -ay
# display all logical volumes
lvs
  LV                                       VG      Attr       LSize  Pool Origin Data%  Meta%  Move Log Cpy%Sync Convert
  pvc-12cec25c-325e-4a89-9cad-15360f870235 csi-lvm Rwi-aor--- 10.00g                                    100.00
  pvc-189851cc-c94f-4d26-8da0-490b0e511fec csi-lvm Rwi-aor--- 10.00g                                    100.00
  pvc-194547f4-8e46-4d31-94f1-654d0ca03378 csi-lvm Rwi-aor--- 10.00g                                    100.00
...
# leave this container
exit
```

1. Mount these logical volumes, `--mount-shared` is required that the kubelet and the pod can access the volume.

```bash
cd /dev/csi-lvm
ls | while read line; do mkdir -p /tmp/csi-lvm/$line || true ; mount --make-shared -t ext4 /dev/csi-lvm/$line /tmp/csi-lvm/$line; done
```

Now all former pv´s should be mounted at the original place, please ensure to restart all pods which have these pv´s mounted before.
