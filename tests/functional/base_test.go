/*
Copyright 2022.

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

package functional_test

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
)

const (
	timeout  = time.Second * 10
	interval = timeout / 100
)

func GetDefaultOVNNorthdSpec() map[string]interface{} {
	return map[string]interface{}{
		"containerImage": "test-ovnnorthd-container-image",
	}
}

func CreateOVNNorthd(namespace string, OVNNorthdName string, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "ovn.openstack.org/v1beta1",
		"kind":       "OVNNorthd",
		"metadata": map[string]interface{}{
			"name":      OVNNorthdName,
			"namespace": namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetOVNNorthd(name types.NamespacedName) *ovnv1.OVNNorthd {
	instance := &ovnv1.OVNNorthd{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func OVNNorthdConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOVNNorthd(name)
	return instance.Status.Conditions
}

func GetDefaultOVNDBClusterSpec() map[string]interface{} {
	return map[string]interface{}{
		"containerImage": "test-ovn-nb-container-image",
		"storageRequest": "1G",
		"storageClass":   "local-storage",
	}
}

func CreateOVNDBCluster(namespace string, OVNDBClusterName string, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "ovn.openstack.org/v1beta1",
		"kind":       "OVNDBCluster",
		"metadata": map[string]interface{}{
			"name":      OVNDBClusterName,
			"namespace": namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func OVNDBClusterConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOVNDBCluster(name)
	return instance.Status.Conditions
}

// CreateOVNDBClusters Creates NB and SB OVNDBClusters
func CreateOVNDBClusters(namespace string) []types.NamespacedName {
	dbs := []types.NamespacedName{}
	for _, db := range []string{"NB", "SB"} {
		ovndbcluster := &ovnv1.OVNDBCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ovn.openstack.org/v1beta1",
				Kind:       "OVNDBCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ovn-" + uuid.New().String(),
				Namespace: namespace,
			},
			Spec: ovnv1.OVNDBClusterSpec{
				DBType:         db,
				StorageRequest: "1G",
			},
		}

		Expect(k8sClient.Create(ctx, ovndbcluster.DeepCopy())).Should(Succeed())
		name := types.NamespacedName{Namespace: namespace, Name: ovndbcluster.Name}

		dbaddr := "tcp:10.1.1.1:6641"
		if db == "SB" {
			dbaddr = "tcp:10.1.1.1:6642"
		}

		// the Status field needs to be written via a separate client
		ovndbcluster = GetOVNDBCluster(name)
		ovndbcluster.Status = ovnv1.OVNDBClusterStatus{
			InternalDBAddress: dbaddr,
		}
		Eventually(func(g Gomega) {
			ovndbcluster = GetOVNDBCluster(name)
			ovndbcluster.Status.InternalDBAddress = dbaddr
			g.Expect(k8sClient.Status().Update(ctx, ovndbcluster)).Should(Succeed())
		}, timeout, interval).Should(Succeed())

		dbs = append(dbs, name)

	}

	logger.Info("OVNDBClusters created", "OVNDBCluster", dbs)
	return dbs
}

// DeleteOVNDBClusters Delete OVN DBClusters
func DeleteOVNDBClusters(names []types.NamespacedName) {
	for _, db := range names {
		th.DeleteInstance(GetOVNDBCluster(db))
	}
}

// GetOVNDBCluster Get OVNDBCluster
func GetOVNDBCluster(name types.NamespacedName) *ovnv1.OVNDBCluster {
	instance := &ovnv1.OVNDBCluster{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetDaemonSet -
func GetDaemonSet(name types.NamespacedName) *appsv1.DaemonSet {
	ds := &appsv1.DaemonSet{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, ds)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return ds
}

// ListDaemonsets -
func ListDaemonsets(namespace string) *appsv1.DaemonSetList {
	dss := &appsv1.DaemonSetList{}
	Expect(k8sClient.List(ctx, dss, client.InNamespace(namespace))).Should(Succeed())
	return dss
}

// SimulateDaemonsetNumberReady -
func SimulateDaemonsetNumberReady(name types.NamespacedName) {
	Eventually(func(g Gomega) {
		ds := GetDaemonSet(name)
		ds.Status.NumberReady = 1
		ds.Status.DesiredNumberScheduled = 1
		g.Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())

	}, timeout, interval).Should(Succeed())
	logger.Info("Simulated daemonset success", "on", name)
}

func GetDefaultOVNControllerSpec() map[string]interface{} {
	return map[string]interface{}{
		// Default external Ids not picked up
		"external-ids": map[string]interface{}{
			"ovn-encap-type": "geneve",
		},
	}
}

func CreateOVNController(namespace string, OVNControllerName string, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "ovn.openstack.org/v1beta1",
		"kind":       "OVNController",
		"metadata": map[string]interface{}{
			"name":      OVNControllerName,
			"namespace": namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetOVNController(name types.NamespacedName) *ovnv1.OVNController {
	instance := &ovnv1.OVNController{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func OVNControllerConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOVNController(name)
	return instance.Status.Conditions
}

func SimulateDaemonsetNumberReadyWithPods(name types.NamespacedName, networkIPs map[string][]string) {
	ds := GetDaemonSet(name)

	for i := 0; i < int(1); i++ {
		pod := &corev1.Pod{
			ObjectMeta: ds.Spec.Template.ObjectMeta,
			Spec:       ds.Spec.Template.Spec,
		}
		pod.ObjectMeta.Namespace = name.Namespace
		pod.ObjectMeta.GenerateName = name.Name
		pod.ObjectMeta.Labels = map[string]string{
			"service": "ovn-controller",
		}

		// NodeName required for getOvsPodsNodes
		pod.Spec.NodeName = name.Name

		var netStatus []networkv1.NetworkStatus
		for network, IPs := range networkIPs {
			netStatus = append(
				netStatus,
				networkv1.NetworkStatus{
					Name: network,
					IPs:  IPs,
				},
			)
		}
		netStatusAnnotation, err := json.Marshal(netStatus)
		Expect(err).NotTo(HaveOccurred())
		pod.Annotations[networkv1.NetworkStatusAnnot] = string(netStatusAnnotation)
		Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
	}

	Eventually(func(g Gomega) {
		ds := GetDaemonSet(name)
		ds.Status.NumberReady = 1
		ds.Status.DesiredNumberScheduled = 1
		g.Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())

	}, timeout, interval).Should(Succeed())

	logger.Info("Simulated daemonset success", "on", name)
}
