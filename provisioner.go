package main

import (
	"fmt"
	"path"

	"sigs.k8s.io/sig-storage-lib-external-provisioner/controller"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/klog"
)

const (
	provisionerName  = "metal-pod.io/lvm"
	keyNode          = "kubernetes.io/hostname"
	actionTypeCreate = "create"
	actionTypeDelete = "delete"
)

type actionType string

type lvmProvisioner struct {
	// The directory to create the directories for every lv and mount them
	lvDir string

	// devicePattern specifies a pattern of host devices to be part of the main volume group
	devicePattern string

	// image to execute lvm commands
	provisionerImage string
}

// NewLVMProvisioner creates a new hostpath provisioner
func NewLVMProvisioner(lvDir, devicePattern, provisionerImage string) controller.Provisioner {
	return &lvmProvisioner{
		lvDir:            lvDir,
		devicePattern:    devicePattern,
		provisionerImage: provisionerImage,
	}
}

var _ controller.Provisioner = &lvmProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *lvmProvisioner) Provision(options controller.ProvisionOptions) (*v1.PersistentVolume, error) {
	node := options.SelectedNode
	if node == nil {
		return nil, fmt.Errorf("configuration error, no node was specified")
	}

	name := options.PVName
	path := path.Join(p.lvDir, name)

	klog.Info("Creating volume %v at %v:%v", name, node.Name, path)

	createCmdsForPath := []string{
		"mkdir",
		"-m", "0777",
		"-p",
	}
	if err := p.createHelperPod(actionTypeCreate, createCmdsForPath, name, path, node.Name); err != nil {
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
	defer func() {
		err = fmt.Errorf("failed to delete volume %v, err:%v", volume.Name, err)
	}()
	path, node, err := p.getPathAndNodeForPV(volume)
	if err != nil {
		return err
	}
	if volume.Spec.PersistentVolumeReclaimPolicy != v1.PersistentVolumeReclaimRetain {
		klog.Info("Deleting volume %v at %v:%v", volume.Name, node, path)
		// FIXME implement for lvm
		cleanupCmdsForPath := []string{"rm", "-rf"}
		if err := p.createHelperPod(actionTypeDelete, cleanupCmdsForPath, volume.Name, path, node); err != nil {
			klog.Info("clean up volume %v failed: %v", volume.Name, err)
			return err
		}
		return nil
	}
	klog.Info("Retained volume %v", volume.Name)
	return nil
}

func (p *lvmProvisioner) createHelperPod(action actionType, cmdsForPath []string, name, path, node string) (err error) {
	defer func() {
		err = fmt.Errorf("failed to %v volume %v err:%v", action, name, err)
	}()
	if name == "" || path == "" || node == "" {
		return fmt.Errorf("invalid empty name or path or node")
	}

	// hostPathType := v1.HostPathDirectoryOrCreate
	// helperPod := &v1.Pod{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: string(action) + "-" + name,
	// 	},
	// 	Spec: v1.PodSpec{
	// 		RestartPolicy: v1.RestartPolicyNever,
	// 		NodeName:      node,
	// 		Tolerations: []v1.Toleration{
	// 			{
	// 				Operator: v1.TolerationOpExists,
	// 			},
	// 		},
	// 		Containers: []v1.Container{
	// 			{
	// 				Name:    "local-path-" + string(action),
	// 				Image:   p.provisionerImage,
	// 				Command: append(cmdsForPath, path.Join("/data/", volumeDir)),
	// 				VolumeMounts: []v1.VolumeMount{
	// 					{
	// 						Name:      "data",
	// 						ReadOnly:  false,
	// 						MountPath: "/data/",
	// 					},
	// 				},
	// 				ImagePullPolicy: v1.PullIfNotPresent,
	// 			},
	// 		},
	// 		Volumes: []v1.Volume{
	// 			{
	// 				Name: "data",
	// 				VolumeSource: v1.VolumeSource{
	// 					HostPath: &v1.HostPathVolumeSource{
	// 						Path: parentDir,
	// 						Type: &hostPathType,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// // If it already exists due to some previous errors, the pod will be cleaned up later automatically
	// // https://github.com/rancher/local-path-provisioner/issues/27
	// _, err = p.kubeClient.CoreV1().Pods(p.namespace).Create(helperPod)
	// if err != nil && !k8serror.IsAlreadyExists(err) {
	// 	return err
	// }

	// defer func() {
	// 	e := p.kubeClient.CoreV1().Pods(p.namespace).Delete(helperPod.Name, &metav1.DeleteOptions{})
	// 	if e != nil {
	// 		logrus.Errorf("unable to delete the helper pod: %v", e)
	// 	}
	// }()

	// completed := false
	// for i := 0; i < CmdTimeoutCounts; i++ {
	// 	if pod, err := p.kubeClient.CoreV1().Pods(p.namespace).Get(helperPod.Name, metav1.GetOptions{}); err != nil {
	// 		return err
	// 	} else if pod.Status.Phase == v1.PodSucceeded {
	// 		completed = true
	// 		break
	// 	}
	// 	time.Sleep(1 * time.Second)
	// }
	// if !completed {
	// 	return fmt.Errorf("create process timeout after %v seconds", CmdTimeoutCounts)
	// }

	klog.Info("Volume %v has been %vd on %v:%v", name, action, node, path)
	return nil
}

func (p *lvmProvisioner) getPathAndNodeForPV(pv *v1.PersistentVolume) (path, node string, err error) {
	defer func() {
		err = fmt.Errorf("failed to delete volume %v err:%v", pv.Name, err)
	}()

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
