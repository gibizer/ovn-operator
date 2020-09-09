/*
Copyright 2020 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stubs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	yaml "sigs.k8s.io/yaml"

	ovncentralv1alpha1 "github.com/openstack-k8s-operators/ovn-central-operator/api/v1alpha1"
)

const (
	dataVolumeName = "data"
)

var (
	ovsdbContainerTpl     corev1.Container
	pvcTpl                corev1.PersistentVolumeClaim
	bootstrapContainerTpl corev1.Container
)

func init() {
	yamlsPath := os.Getenv("OPERATOR_YAMLS")
	if yamlsPath == "" {
		panic("Environment variable OPERATOR_YAMLS is not set")
	}

	yamls := map[string]interface{}{
		"ovsdb_container.yaml":     &ovsdbContainerTpl,
		"pvc.yaml":                 &pvcTpl,
		"bootstrap-container.yaml": &bootstrapContainerTpl,
	}

	for file, obj := range yamls {
		contents, err := ioutil.ReadFile(path.Join(yamlsPath, file))
		if err != nil {
			panic(err)
		}

		err = yaml.UnmarshalStrict(contents, obj)
		if err != nil {
			panic(err)
		}
	}
}

func OVSDBContainer(cr *ovncentralv1alpha1.OVNCentralSpec,
	containerName string, containerCommand []string) *corev1.Container {

	container := ovsdbContainerTpl.DeepCopy()
	container.Image = cr.Image
	container.Name = containerName
	container.Command = containerCommand

	return container
}

func commonLabels(cr *ovncentralv1alpha1.OVNCentral) map[string]string {
	return map[string]string{
		"app":         "ovn-central",
		"ovn-central": cr.Name,
	}
}

func ServerService(cr *ovncentralv1alpha1.OVNCentral, scheme *runtime.Scheme, index int) *corev1.Service {
	service := &corev1.Service{}

	service.Name = fmt.Sprintf("%s-%v", cr.Name, index)
	service.Namespace = cr.Namespace
	service.Labels = commonLabels(cr)

	service.Spec.Selector = commonLabels(cr)
	service.Spec.Selector["statefulset.kubernetes.io/pod-name"] = service.Name

	service.Spec.Ports = []corev1.ServicePort{
		{Name: "north", Port: 6641},
		{Name: "south", Port: 6642},
		{Name: "north-raft", Port: 6643},
		{Name: "south-raft", Port: 6644},
	}

	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.SessionAffinity = corev1.ServiceAffinityNone

	controllerutil.SetControllerReference(cr, service, scheme)

	return service
}

func PVC(
	cr *ovncentralv1alpha1.OVNCentral,
	scheme *runtime.Scheme,
	index int) *corev1.PersistentVolumeClaim {

	pvc := pvcTpl.DeepCopy()

	pvc.Name = fmt.Sprintf("%s-%s-%d", dataVolumeName, cr.Name, index)
	pvc.Namespace = cr.Namespace
	pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: cr.Spec.StorageSize}
	pvc.Spec.StorageClassName = cr.Spec.StorageClass

	controllerutil.SetControllerReference(cr, pvc, scheme)

	return pvc
}

func BootstrapPod(
	cr *ovncentralv1alpha1.OVNCentral,
	scheme *runtime.Scheme,
	pvc *corev1.PersistentVolumeClaim) *corev1.Pod {

	bootstrapPod := &corev1.Pod{}
	bootstrapPod.Name = fmt.Sprintf("%s-bootstrap", cr.Name)
	bootstrapPod.Namespace = cr.Namespace

	podSpec := &bootstrapPod.Spec
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name:         dataVolumeName,
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}},
	})

	nb_address := fmt.Sprintf("tcp:%s-0.%s.svc.cluster.local:6641", cr.Name, cr.Namespace)
	nbContainer := bootstrapContainerTpl.DeepCopy()
	nbContainer.Name = "bootstrap-nb"
	nbContainer.Image = cr.Spec.Image
	nbContainer.Env = append(nbContainer.Env,
		corev1.EnvVar{Name: "DB_TYPE", Value: "NB"},
		corev1.EnvVar{Name: "ADDRESS", Value: nb_address})

	sb_address := fmt.Sprintf("tcp:%s-0.%s.svc.cluster.local:6642", cr.Name, cr.Namespace)
	sbContainer := bootstrapContainerTpl.DeepCopy()
	sbContainer.Name = "bootstrap-sb"
	sbContainer.Image = cr.Spec.Image
	sbContainer.Env = append(nbContainer.Env,
		corev1.EnvVar{Name: "DB_TYPE", Value: "SB"},
		corev1.EnvVar{Name: "ADDRESS", Value: sb_address})

	podSpec.Containers = []corev1.Container{*nbContainer, *sbContainer}

	controllerutil.SetControllerReference(cr, bootstrapPod, scheme)

	return bootstrapPod
}
