package main

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/controller"

	"k8s.io/klog"
)

const (
	keyNode           = "kubernetes.io/hostname"
	stripedAnnotation = "striped.metal-pod.io/csi-lvm"
	actionTypeCreate  = "create"
	actionTypeDelete  = "delete"
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
}

// NewLVMProvisioner creates a new hostpath provisioner
func NewLVMProvisioner(kubeClient clientset.Interface, namespace, lvDir, devicePattern, provisionerImage string) controller.Provisioner {
	return &lvmProvisioner{
		lvDir:            lvDir,
		devicePattern:    devicePattern,
		provisionerImage: provisionerImage,
		kubeClient:       kubeClient,
		namespace:        namespace,
	}
}

var _ controller.Provisioner = &lvmProvisioner{}

type volumeAction struct {
	action   actionType
	name     string
	path     string
	nodeName string
	size     int64
	striped  bool
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *lvmProvisioner) Provision(options controller.ProvisionOptions) (*v1.PersistentVolume, error) {
	node := options.SelectedNode
	if node == nil {
		return nil, fmt.Errorf("configuration error, no node was specified")
	}

	name := options.PVName
	path := path.Join(p.lvDir, name)

	striped := true
	var err error
	a, ok := options.PVC.Annotations[stripedAnnotation]
	if ok {
		striped, err = strconv.ParseBool(a)
		if err != nil {
			klog.Errorf("striped annotation must be either 'true|false' but is %s error:%v", stripedAnnotation, err)
		}
	}

	klog.Infof("Creating volume %v at %v:%v", name, node.Name, path)
	requests, ok := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	if !ok {
		return nil, fmt.Errorf("configuration error, no volume size was specified")
	}
	size, ok := requests.AsInt64()
	if !ok {
		return nil, fmt.Errorf("configuration error, no volume size not readable")
	}

	va := volumeAction{
		action:   actionTypeCreate,
		name:     name,
		path:     path,
		nodeName: node.Name,
		size:     size,
		striped:  striped,
	}
	if err := p.createProvisionerPod(va); err != nil {
		klog.Errorf("error creating provisioner pod :%v", err)
		return nil, err
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
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
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

	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *lvmProvisioner) Delete(volume *v1.PersistentVolume) (err error) {
	path, node, err := p.getPathAndNodeForPV(volume)
	if err != nil {
		return err
	}
	if volume.Spec.PersistentVolumeReclaimPolicy != v1.PersistentVolumeReclaimRetain {
		klog.Infof("Deleting volume %v at %v:%v", volume.Name, node, path)
		va := volumeAction{
			action:   actionTypeDelete,
			name:     volume.Name,
			path:     path,
			nodeName: node,
			size:     0,
		}
		if err := p.createProvisionerPod(va); err != nil {
			klog.Infof("clean up volume %v failed: %v", volume.Name, err)
			return err
		}
		return nil
	}
	klog.Infof("Retained volume %v", volume.Name)
	return nil
}

func (p *lvmProvisioner) createProvisionerPod(va volumeAction) (err error) {
	if va.name == "" || va.path == "" || va.nodeName == "" {
		return fmt.Errorf("invalid empty name or path or node")
	}

	args := []string{}
	if va.action == actionTypeCreate {
		args = append(args, "createlv", "--lvsize", fmt.Sprintf("%d", va.size), "--devices", p.devicePattern)
		if va.striped {
			args = append(args, "--striped")
		}
	}
	if va.action == actionTypeDelete {
		args = append(args, "deletelv")
	}
	args = append(args, "--lvname", va.name, "--vgname", "csi-lvm", "--directory", p.lvDir)

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
					Name:  "csi-lvm-" + string(va.action),
					Image: p.provisionerImage,
					Args:  args,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "data",
							ReadOnly:         false,
							MountPath:        "/data",
							MountPropagation: &mountPropagation,
						},
						{
							Name:      "devices",
							ReadOnly:  false,
							MountPath: "/dev",
						},
					},
					// FIXME set to always
					ImagePullPolicy: v1.PullIfNotPresent,
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
			},
		},
	}

	// If it already exists due to some previous errors, the pod will be cleaned up later automatically
	// https://github.com/rancher/local-path-provisioner/issues/27
	_, err = p.kubeClient.CoreV1().Pods(p.namespace).Create(provisionerPod)
	if err != nil && !k8serror.IsAlreadyExists(err) {
		return err
	}

	defer func() {
		e := p.kubeClient.CoreV1().Pods(p.namespace).Delete(provisionerPod.Name, &metav1.DeleteOptions{})
		if e != nil {
			klog.Errorf("unable to delete the provisioner pod: %v", e)
		}
	}()

	completed := false
	for i := 0; i < 20; i++ {
		if pod, err := p.kubeClient.CoreV1().Pods(p.namespace).Get(provisionerPod.Name, metav1.GetOptions{}); err != nil {
			logs := getPodLogs(p.kubeClient, pod)
			klog.Errorf("provisioner pod logs: %s", logs)
			return err
		} else if pod.Status.Phase == v1.PodSucceeded {
			completed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !completed {
		return fmt.Errorf("create process timeout after %v seconds", 20)
	}

	klog.Infof("Volume %v has been %vd on %v:%v", va.name, va.action, va.nodeName, va.path)
	return nil
}

func getPodLogs(kubeClient clientset.Interface, pod *v1.Pod) string {
	podLogOpts := v1.PodLogOptions{}
	req := kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream()
	if err != nil {
		return "error in opening stream"
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "error in copy information from podLogs to buf"
	}
	str := buf.String()
	return str
}

func (p *lvmProvisioner) getPathAndNodeForPV(pv *v1.PersistentVolume) (path, node string, err error) {
	hostPath := pv.Spec.PersistentVolumeSource.HostPath
	if hostPath == nil {
		return "", "", fmt.Errorf("no HostPath set")
	}
	path = hostPath.Path

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
