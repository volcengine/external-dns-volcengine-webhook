// Copyright 2025 The Beijing Volcano Engine Technology Co., Ltd. Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

// KubernetesClient encapsulates operations on Kubernetes resources
type KubernetesClient struct {
	clientset *kubernetes.Clientset
}

// NewKubernetesClient creates a new Kubernetes client
func NewKubernetesClient(kubeconfig string) (*KubernetesClient, error) {
	// Write kubeconfig to a temporary file
	tempFile, err := os.CreateTemp("", "kubeconfig-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for kubeconfig: %w", err)
	}
	_ = tempFile.Close()

	// Add defer statement to ensure the temporary file is deleted
	defer func() {
		_ = os.Remove(tempFile.Name())
	}()

	if err := os.WriteFile(tempFile.Name(), []byte(kubeconfig), 0600); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig to temp file: %w", err)
	}

	// Create config from file
	config, err := clientcmd.BuildConfigFromFlags("", tempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &KubernetesClient{clientset: clientset}, nil
}

// CreateTestService creates a test Service resource
func (k *KubernetesClient) CreateTestService(ctx context.Context, namespace, name, domain string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": domain,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			Selector: map[string]string{
				"app": name,
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	_, err := k.clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	return err
}

// CreateTestServiceWithCNAME creates a test Service with CNAME record
func (k *KubernetesClient) CreateTestServiceWithCNAME(ctx context.Context, namespace, name, domain, cnameTarget string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": domain,
				"external-dns.alpha.kubernetes.io/ttl":      "300",
				"external-dns.alpha.kubernetes.io/type":     "CNAME",
				"external-dns.alpha.kubernetes.io/target":   cnameTarget,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			Selector: map[string]string{
				"app": name,
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	_, err := k.clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	return err
}

// CreateTestIngress creates a test Ingress resource
func (k *KubernetesClient) CreateTestIngress(ctx context.Context, namespace, name, domain string) error {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": domain,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{
				{
					Host: domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: func() *networkingv1.PathType { pathType := networkingv1.PathTypePrefix; return &pathType }(),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: name,
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := k.clientset.NetworkingV1().Ingresses(namespace).Create(ctx, ingress, metav1.CreateOptions{})
	return err
}

// DeleteTestResources deletes test-created resources
func (k *KubernetesClient) DeleteTestResources(ctx context.Context, namespace, name string) error {
	// Delete Ingress
	if err := k.clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ingress: %w", err)
		}
	}

	// Delete Service
	if err := k.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		fmt.Printf("failed to delete service: %s\n", err)
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete service: %w", err)
		}
	}

	return nil
}

// WaitForDNSRecord continuously queries PrivateZone, waiting for DNS record creation to complete
func (k *KubernetesClient) WaitForDNSRecord(ctx context.Context, pzClient *PrivateZoneClient, zoneID int64, host string, timeout time.Duration) (bool, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			return false, fmt.Errorf("timeout waiting for DNS record %s", host)
		case <-ticker.C:
			records, err := pzClient.ListRecords(ctx, zoneID)
			if err != nil {
				return false, err
			}

			for _, record := range records {
				if *record.Host == host {
					return true, nil
				}
			}
		}
	}
}

// CreateNamespace creates a namespace
func (k *KubernetesClient) CreateNamespace(ctx context.Context, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"name": namespace,
			},
		},
	}

	// Check if namespace already exists
	_, err := k.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists, no need to create
		return nil
	}

	// If error is due to not found, create the namespace
	if _, err := k.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}

	return nil
}

// DeleteNamespace deletes a namespace
func (k *KubernetesClient) DeleteNamespace(ctx context.Context, namespace string) error {
	// Delete namespace
	err := k.clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace %s: %w", namespace, err)
	}

	// Wait for namespace to be completely deleted
	timeout := 2 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		_, err := k.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			// If not found error is returned, namespace has been deleted
			if errors.IsNotFound(err) {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for namespace %s to be deleted", namespace)
}

// UpdateTestService updates test Service resource annotations
func (k *KubernetesClient) UpdateTestService(ctx context.Context, namespace, name, newDomain string, newTTL string, newTarget string) error {
	// Get existing Service
	svc, err := k.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	// Update annotations
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}

	if newDomain != "" {
		svc.Annotations["external-dns.alpha.kubernetes.io/hostname"] = newDomain
	}

	if newTTL != "" {
		svc.Annotations["external-dns.alpha.kubernetes.io/ttl"] = newTTL
	}

	if newTarget != "" {
		svc.Annotations["external-dns.alpha.kubernetes.io/target"] = newTarget
	}

	// Update Service
	_, err = k.clientset.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	return err
}

// UpdateTestIngress updates test Ingress resource annotations
func (k *KubernetesClient) UpdateTestIngress(ctx context.Context, namespace, name, oldDomain, newDomain string, newTTL string, newTarget string) error {
	// Get existing Ingress
	ingress, err := k.clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	// Update annotations
	if ingress.Annotations == nil {
		ingress.Annotations = make(map[string]string)
	}

	if newDomain != "" {
		ingress.Annotations["external-dns.alpha.kubernetes.io/hostname"] = newDomain
		for i, rule := range ingress.Spec.Rules {
			if oldDomain != "" && rule.Host == oldDomain {
				ingress.Spec.Rules[i].Host = newDomain
			}
		}
	}

	if newTTL != "" {
		ingress.Annotations["external-dns.alpha.kubernetes.io/ttl"] = newTTL
	}

	if newTarget != "" {
		ingress.Annotations["external-dns.alpha.kubernetes.io/target"] = newTarget
	}

	// Update Ingress
	_, err = k.clientset.NetworkingV1().Ingresses(namespace).Update(ctx, ingress, metav1.UpdateOptions{})
	return err
}

// CreateTestExternalNameService creates an ExternalName type test Service for controlling CNAME records
func (k *KubernetesClient) CreateTestExternalNameService(ctx context.Context, namespace, name, domain, externalName string) error {
	// Create a regular Service with CNAME annotation
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": domain,
				"external-dns.alpha.kubernetes.io/ttl":      "300",
				"external-dns.alpha.kubernetes.io/type":     "CNAME",
				"external-dns.alpha.kubernetes.io/target":   externalName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: externalName,
		},
	}

	_, err := k.clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	return err
}

// UpdateTestExternalNameService updates externalName of an existing ExternalName type Service
func (k *KubernetesClient) UpdateTestExternalNameService(ctx context.Context, namespace, name, domain, newExternalName string) error {
	// Get existing Service
	svc, err := k.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	// Update Service's externalName and annotations
	svc.Spec.ExternalName = newExternalName
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations["external-dns.alpha.kubernetes.io/target"] = newExternalName

	// Update Service using Update method
	_, err = k.clientset.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ExternalName Service: %w", err)
	}

	return nil
}

// WaitForDNSRecordUpdate waits for DNS record update, can check value, TTL, or both
// expectedValue: expected record value, if "" then not checking value
// expectedTTL: expected TTL value, if 0 then not checking TTL
// timeout: timeout duration
func (k *KubernetesClient) WaitForDNSRecordUpdate(ctx context.Context, pzClient *PrivateZoneClient, zoneID int64, host, recordType string, expectedValue string, expectedTTL int32, timeout time.Duration) (bool, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	checkValue := expectedValue != ""
	checkTTL := expectedTTL != 0

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			return false, fmt.Errorf("timeout waiting for DNS record update")
		case <-ticker.C:
			record, err := pzClient.GetRecordByHostAndType(ctx, zoneID, host, recordType)
			if err != nil {
				// If record doesn't exist, continue waiting
				continue
			}

			// Check if record meets expected conditions
			match := true

			if checkValue {
				if recordType == "CNAME" {
					// For CNAME records, need to remove trailing dot
					if strings.TrimSuffix(*record.Value, ".") != expectedValue {
						match = false
					}
				} else {
					// For other record types, directly compare values
					if *record.Value != expectedValue {
						match = false
					}
				}
			}

			if checkTTL && *record.TTL != expectedTTL {
				match = false
			}

			if match {
				return true, nil
			}
		}
	}
}
