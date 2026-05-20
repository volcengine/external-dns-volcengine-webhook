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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type trackedDNSRecord struct {
	host       string
	recordType string
}

var _ = Describe("Cross-account ExternalDNS provider", Label("cross-account"), func() {
	var (
		config         *TestConfig
		kubeClient     *KubernetesClient
		pzClient       *PrivateZoneClient
		testZoneID     int64
		testDomain     string
		testNamespace  = "external-dns-cross-account-e2e"
		testName       = "cross-account-test-app"
		trackedRecords []trackedDNSRecord
	)

	resetNamespace := func(ctx context.Context) {
		By("Resetting cross-account test namespace")
		_ = kubeClient.DeleteNamespace(ctx, testNamespace)
		time.Sleep(3 * time.Second)
		Expect(kubeClient.CreateNamespace(ctx, testNamespace)).NotTo(HaveOccurred(), "Failed to create cross-account test namespace")
	}

	trackRecord := func(host, recordType string) {
		trackedRecords = append(trackedRecords, trackedDNSRecord{host: host, recordType: recordType})
	}

	BeforeEach(func() {
		var err error
		config, err = LoadCrossAccountTestConfig()
		Expect(err).NotTo(HaveOccurred(), "Failed to load cross-account test config")

		if !config.CrossAccountEnabled {
			Skip("Cross-account e2e is disabled; set VOLCENGINE_E2E_CROSS_ACCOUNT=true to enable it")
		}
		if config.DomainName == "" {
			Skip("Cross-account ExternalDNS e2e requires TEST_DOMAIN_NAME")
		}
		if config.ClusterID == "" && config.ClusterName == "" {
			Skip("Cross-account ExternalDNS e2e requires VOLCENGINE_CLUSTER_ID or VOLCENGINE_CLUSTER_NAME")
		}

		testDomain = config.DomainName
		testZoneID, err = strconv.ParseInt(config.PrivateZoneID, 10, 64)
		Expect(err).NotTo(HaveOccurred(), "Failed to parse private zone ID")

		pzClient, err = NewPrivateZoneClient(config)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cross-account privatezone client")

		kubeconfig, err := GetClusterKubeconfig(config)
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster kubeconfig")

		kubeClient, err = NewKubernetesClient(kubeconfig)
		Expect(err).NotTo(HaveOccurred(), "Failed to create kubernetes client")

		trackedRecords = nil
		resetNamespace(context.Background())
	})

	AfterEach(func() {
		if kubeClient == nil || pzClient == nil {
			return
		}

		ctx := context.Background()
		By("Cleaning up Kubernetes resources for cross-account ExternalDNS e2e")
		err := kubeClient.DeleteTestResources(ctx, testNamespace, testName)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete cross-account test resources")

		By("Cleaning up tracked records in the target account")
		for _, record := range trackedRecords {
			Expect(cleanupRecordsByHostAndType(ctx, pzClient, testZoneID, record.host, record.recordType)).
				NotTo(HaveOccurred(), "Failed to cleanup tracked cross-account record")
		}
	})

	It("should create and delete service DNS records in the target account", func() {
		ctx := context.Background()
		host := fmt.Sprintf("cross-service-%d", time.Now().UnixNano())
		domain := fmt.Sprintf("%s.%s", host, testDomain)
		trackRecord(host, "A")

		By("Creating Service with external-dns annotation in the source cluster")
		err := kubeClient.CreateTestService(ctx, testNamespace, testName, domain)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cross-account test Service")

		By("Waiting for the target account DNS record to be created")
		success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for cross-account DNS record")
		Expect(success).To(BeTrue(), "Cross-account DNS record was not created within timeout")

		By("Verifying the target account contains the new A record")
		record, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
		Expect(err).NotTo(HaveOccurred(), "Failed to get cross-account DNS record")
		Expect(record).NotTo(BeNil(), "Cross-account DNS record should not be nil")

		By("Deleting the Service to trigger record cleanup in the target account")
		err = kubeClient.DeleteTestResources(ctx, testNamespace, testName)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete cross-account Service")

		By("Waiting for the target account DNS record to be deleted")
		deleted, err := pzClient.WaitForRecordDeleted(ctx, testZoneID, host, "A", 2*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for cross-account DNS record deletion")
		Expect(deleted).To(BeTrue(), "Cross-account DNS record was not deleted within timeout")
	})

	It("should create ingress DNS records in the target account", func() {
		ctx := context.Background()
		host := fmt.Sprintf("cross-ingress-%d", time.Now().UnixNano())
		domain := fmt.Sprintf("%s.%s", host, testDomain)
		trackRecord(host, "A")

		By("Creating Ingress with external-dns annotation in the source cluster")
		err := kubeClient.CreateTestIngress(ctx, testNamespace, testName, domain)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cross-account test Ingress")

		By("Waiting for the target account DNS record to be created")
		success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "Error waiting for cross-account Ingress DNS record")
		Expect(success).To(BeTrue(), "Cross-account Ingress DNS record was not created within timeout")

		By("Verifying the target account contains the new Ingress A record")
		record, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
		Expect(err).NotTo(HaveOccurred(), "Failed to get cross-account Ingress DNS record")
		Expect(record).NotTo(BeNil(), "Cross-account Ingress DNS record should not be nil")
	})
})
