package main

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v8/controller"

	"k8s.io/klog/v2"
)

const (
	keyNode          = "kubernetes.io/hostname"
	typeAnnotation   = "csi-lvm.metal-stack.io/type"
	linearType       = "linear"
	stripedType      = "striped"
	mirrorType       = "mirror"
	actionTypeCreate = "create"
	actionTypeDelete = "delete"
	pullAlways       = "always"
	pullIfNotPresent = "ifnotpresent"
)

type actionType string

type lvmProvisioner struct {
	// The directory to create the directories for every lv and mount them
	lvDir string
	// devicePattern specifies a pattern of host devices to be part of the main volume group
	devicePattern string
	// image to execute lvm commands
	provisionerImage string
	kubeClient       clientset.Interface
	namespace        string
	// defaultLVMType the lvm type to use by default if not overwritten in the pvc spec.
	defaultLVMType string
	pullPolicy     v1.PullPolicy
	vgName         string
}

// NewLVMProvisioner creates a new lvm provisioner
func NewLVMProvisioner(kubeClient clientset.Interface, namespace, vgName, lvDir, devicePattern, provisionerImage, defaultLVMType, pullPolicy string) controller.Provisioner {
	pp := v1.PullAlways
	if strings.ToLower(pullPolicy) == pullIfNotPresent {
		pp = v1.PullIfNotPresent
	}

	return &lvmProvisioner{
		lvDir:            lvDir,
		devicePattern:    devicePattern,
		provisionerImage: provisionerImage,
		kubeClient:       kubeClient,
		namespace:        namespace,
		vgName:           vgName,
		defaultLVMType:   defaultLVMType,
		pullPolicy:       pp,
	}
}

var _ controller.Provisioner = &lvmProvisioner{}

type volumeAction struct {
	action   actionType
	name     string
	path     string
	nodeName string
	size     int64
	lvmType  string
	isBlock  bool
}

// SupportsBlock returns whether provisioner supports block volume.
// this is required for mixed setups where pv´s mounted in pods
// and block devices must be possible on the same node
func (p *lvmProvisioner) SupportsBlock(ctx context.Context) bool {
	return true
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *lvmProvisioner) Provision(ctx context.Context, options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	klog.Infof("start provision %s node:%s devices:%s", options.PVName, options.SelectedNode.GetName(), p.devicePattern)
	node := options.SelectedNode
	if node == nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("configuration error, no node was specified")
	}

	name := options.PVName
	path := path.Join(p.lvDir, name)

	lvmType := p.defaultLVMType

	userAnnotation, ok := options.PVC.Annotations[typeAnnotation]
	if ok {
		lvmType = userAnnotation
	}

	switch lvmType {
	case stripedType, mirrorType, linearType:
	default:
		return nil, controller.ProvisioningFinished, fmt.Errorf("configuration error, lvmtype %s is invalid", lvmType)
	}

	klog.Infof("Creating volume %v at %v:%v", name, node.Name, path)
	requests, ok := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	if !ok {
		return nil, controller.ProvisioningFinished, fmt.Errorf("configuration error, no volume size was specified")
	}
	size, ok := requests.AsInt64()
	if !ok {
		return nil, controller.ProvisioningFinished, fmt.Errorf("configuration error, no volume size not readable")
	}

	volumeMode := v1.PersistentVolumeFilesystem
	isBlock := false
	if options.PVC.Spec.VolumeMode != nil && *options.PVC.Spec.VolumeMode == v1.PersistentVolumeBlock {
		isBlock = true
		volumeMode = v1.PersistentVolumeBlock
	}

	va := volumeAction{
		action:   actionTypeCreate,
		name:     name,
		path:     path,
		nodeName: node.Name,
		size:     size,
		lvmType:  lvmType,
		isBlock:  isBlock,
	}
	if err := p.createProvisionerPod(ctx, va); err != nil {
		klog.Errorf("error creating provisioner pod :%v", err)
		return nil, controller.ProvisioningReschedule, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				"lvmProvisionerIdentity": node.Name,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				Local: &v1.LocalVolumeSource{
					Path: path,
				},
			},
			VolumeMode: &volumeMode,
			NodeAffinity: &v1.VolumeNodeAffinity{
				Required: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      keyNode,
									Operator: v1.NodeSelectorOpIn,
									Values: []string{
										node.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return pv, controller.ProvisioningFinished, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *lvmProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) (err error) {
	path, node, err := p.getPathAndNodeForPV(volume)
	if err != nil {
		return err
	}

	_, err = p.kubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			klog.Infof("node %s not found anymore. Assuming volume %s is gone for good.", node, volume.Name)
			return nil
		}
	}

	klog.Infof("delete volume: %s on node:%s reclaim:%s", path, node, volume.Spec.PersistentVolumeReclaimPolicy)
	if volume.Spec.PersistentVolumeReclaimPolicy != v1.PersistentVolumeReclaimRetain {
		isBlock := false
		if volume.Spec.VolumeMode != nil && *volume.Spec.VolumeMode == v1.PersistentVolumeBlock {
			isBlock = true
		}

		klog.Infof("deleting volume %v at %v:%v", volume.Name, node, path)
		va := volumeAction{
			action:   actionTypeDelete,
			name:     volume.Name,
			path:     path,
			nodeName: node,
			size:     0,
			isBlock:  isBlock,
		}
		if err := p.createProvisionerPod(ctx, va); err != nil {
			klog.Infof("clean up volume %v failed: %v", volume.Name, err)
			return err
		}
		return nil
	}
	klog.Infof("Retained volume %v", volume.Name)
	return nil
}

func (p *lvmProvisioner) createProvisionerPod(ctx context.Context, va volumeAction) (err error) {
	if va.name == "" || va.path == "" || va.nodeName == "" {
		return fmt.Errorf("invalid empty name or path or node")
	}
	if va.action == actionTypeCreate && va.lvmType == "" {
		return fmt.Errorf("createlv without lvm type")
	}

	args := []string{}
	if va.action == actionTypeCreate {
		args = append(args, "createlv", "--lvsize", fmt.Sprintf("%d", va.size), "--devices", p.devicePattern, "--lvmtype", va.lvmType)
	}
	if va.action == actionTypeDelete {
		args = append(args, "deletelv")
	}
	args = append(args, "--lvname", va.name, "--vgname", p.vgName, "--directory", p.lvDir)
	if va.isBlock {
		args = append(args, "--block")
	}

	klog.Infof("start provisionerPod with args:%s", args)
	hostPathType := v1.HostPathDirectoryOrCreate
	mountPropagation := v1.MountPropagationBidirectional
	privileged := true
	provisionerPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(va.action) + "-" + va.name,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			NodeName:      va.nodeName,
			Tolerations: []v1.Toleration{
				{
					Operator: v1.TolerationOpExists,
				},
			},
			Containers: []v1.Container{
				{
					Name:    "csi-lvm-" + string(va.action),
					Image:   p.provisionerImage,
					Command: []string{"/csi-lvm-provisioner"},
					Args:    args,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "data",
							ReadOnly:         false,
							MountPath:        p.lvDir,
							MountPropagation: &mountPropagation,
						},
						{
							Name:      "devices",
							ReadOnly:  false,
							MountPath: "/dev",
						},
						{
							Name:      "modules",
							ReadOnly:  false,
							MountPath: "/lib/modules",
						},
						{
							Name:             "lvmbackup",
							ReadOnly:         false,
							MountPath:        "/etc/lvm/backup",
							MountPropagation: &mountPropagation,
						},
						{
							Name:             "lvmcache",
							ReadOnly:         false,
							MountPath:        "/etc/lvm/cache",
							MountPropagation: &mountPropagation,
						},
						{
							Name:             "lvmlock",
							ReadOnly:         false,
							MountPath:        "/run/lock/lvm",
							MountPropagation: &mountPropagation,
						},
					},
					ImagePullPolicy: p.pullPolicy,
					SecurityContext: &v1.SecurityContext{
						Privileged: &privileged,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"cpu":    resource.MustParse("50m"),
							"memory": resource.MustParse("50Mi"),
						},
						Limits: v1.ResourceList{
							"cpu":    resource.MustParse("100m"),
							"memory": resource.MustParse("100Mi"),
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "data",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: p.lvDir,
							Type: &hostPathType,
						},
					},
				},
				{
					Name: "devices",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/dev",
							Type: &hostPathType,
						},
					},
				},
				{
					Name: "modules",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/lib/modules",
							Type: &hostPathType,
						},
					},
				},
				{
					Name: "lvmbackup",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/etc/lvm/backup",
							Type: &hostPathType,
						},
					},
				},
				{
					Name: "lvmcache",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/etc/lvm/cache",
							Type: &hostPathType,
						},
					},
				},
				{
					Name: "lvmlock",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/run/lock/lvm",
							Type: &hostPathType,
						},
					},
				},
			},
		},
	}

	// If it already exists due to some previous errors, the pod will be cleaned up later automatically
	// https://github.com/rancher/local-path-provisioner/issues/27
	_, err = p.kubeClient.CoreV1().Pods(p.namespace).Create(ctx, provisionerPod, metav1.CreateOptions{})
	if err != nil && !k8serror.IsAlreadyExists(err) {
		return err
	}

	defer func() {
		e := p.kubeClient.CoreV1().Pods(p.namespace).Delete(ctx, provisionerPod.Name, metav1.DeleteOptions{})
		if e != nil {
			klog.Errorf("unable to delete the provisioner pod: %v", e)
		}
	}()

	completed := false
	retrySeconds := 120
	for i := 0; i < retrySeconds; i++ {
		pod, err := p.kubeClient.CoreV1().Pods(p.namespace).Get(ctx, provisionerPod.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("error reading provisioner pod:%v", err)
		} else if pod.Status.Phase == v1.PodSucceeded {
			klog.Info("provisioner pod terminated successfully")
			completed = true
			break
		}
		klog.Infof("provisioner pod status:%s", pod.Status.Phase)
		time.Sleep(1 * time.Second)
	}
	if !completed {
		return fmt.Errorf("create process timeout after %v seconds", retrySeconds)
	}

	klog.Infof("Volume %v has been %vd on %v:%v", va.name, va.action, va.nodeName, va.path)
	return nil
}

func (p *lvmProvisioner) getPathAndNodeForPV(pv *v1.PersistentVolume) (path, node string, err error) {
	localPvc := pv.Spec.PersistentVolumeSource.Local
	if localPvc == nil {
		return "", "", fmt.Errorf("no local PVC set")
	}
	path = localPvc.Path

	nodeAffinity := pv.Spec.NodeAffinity
	if nodeAffinity == nil {
		return "", "", fmt.Errorf("no NodeAffinity set")
	}
	required := nodeAffinity.Required
	if required == nil {
		return "", "", fmt.Errorf("no NodeAffinity.Required set")
	}

	node = ""
	for _, selectorTerm := range required.NodeSelectorTerms {
		for _, expression := range selectorTerm.MatchExpressions {
			if expression.Key == keyNode && expression.Operator == v1.NodeSelectorOpIn {
				if len(expression.Values) != 1 {
					return "", "", fmt.Errorf("multiple values for the node affinity")
				}
				node = expression.Values[0]
				break
			}
		}
		if node != "" {
			break
		}
	}
	if node == "" {
		return "", "", fmt.Errorf("cannot find affinited node")
	}
	return path, node, nil
}
