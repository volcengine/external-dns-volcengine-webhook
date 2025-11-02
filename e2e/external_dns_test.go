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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExternalDNS Volcengine Provider", func() {
	var (
		config        *TestConfig
		kubeClient    *KubernetesClient
		pzClient      *PrivateZoneClient
		testDomain    string
		testZoneID    int64
		testNamespace = "external-dns-e2e"
		testName      = "test-app"
	)

	BeforeEach(func() {
		var err error
		config, err = LoadTestConfig()
		Expect(err).NotTo(HaveOccurred(), "Failed to load test config")
		By("Loading test config: " + fmt.Sprintf("%+v", config))

		pzClient, err = NewPrivateZoneClient(config)
		Expect(err).NotTo(HaveOccurred(), "Failed to create privatezone client")

		kubeconfig, err := GetClusterKubeconfig(config)
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster kubeconfig")

		kubeClient, err = NewKubernetesClient(kubeconfig)
		Expect(err).NotTo(HaveOccurred(), "Failed to create kubernetes client")

		if config.DomainName != "" {
			testDomain = config.DomainName
		} else {
			Expect(config.DomainName).NotTo(BeEmpty(), "DomainName must be provided")
		}

		if config.PrivateZoneID != "" {
			testZoneID, err = strconv.ParseInt(config.PrivateZoneID, 10, 64)
			Expect(err).NotTo(HaveOccurred(), "Failed to parse private zone ID")
		} else {
			// If no PrivateZoneID is provided, report an error directly
			Expect(config.PrivateZoneID).NotTo(BeEmpty(), "PrivateZoneID must be provided")
		}

		By("Starting environment cleanup - deleting possible existing namespace with the same name")
		ctx := context.Background()

		// Try to delete existing namespace (ignore error if not exists)
		deleteErr := kubeClient.DeleteNamespace(ctx, testNamespace)
		if deleteErr != nil {
			fmt.Printf("Warning: Failed to delete existing namespace %s, but continuing test: %v\n", testNamespace, deleteErr)
		}

		By("Waiting for namespace deletion to complete")
		// Wait for a while to ensure namespace deletion is complete
		time.Sleep(3 * time.Second)

		By("Creating new test namespace")
		err = kubeClient.CreateNamespace(ctx, testNamespace)
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

		By("Cleaning up possible existing test records")
		err = pzClient.CleanupRecordsForDomain(ctx, testZoneID, testDomain)
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup existing records")
	})

	AfterEach(func() {
		By("Cleaning up test resources")
		ctx := context.Background()

		By("Deleting Kubernetes resources")
		err := kubeClient.DeleteTestResources(ctx, testNamespace, testName)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete test resources")

		By("Cleaning up DNS records")
		err = pzClient.CleanupRecordsForDomain(ctx, testZoneID, testDomain)
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup DNS records")
	})

	Describe("Basic DNS record creation tests", func() {
		It("should create DNS records for a Service with external-dns annotation", func() {
			ctx := context.Background()

			By("Preparing test domain")
			// Create subdomain based on testDomain
			host := "service"
			serviceDomain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Service with external-dns annotation")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, serviceDomain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test Service")

			By("Waiting for external-dns to process and create DNS record")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for DNS record")
			Expect(success).To(BeTrue(), "DNS record was not created within timeout")

			By("Verifying DNS record is correctly created")
			records, err := pzClient.ListRecords(ctx, testZoneID)
			Expect(err).NotTo(HaveOccurred(), "Failed to list DNS records")

			found := false
			for _, record := range records {
				if *record.Host == host && *record.Type == "A" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "DNS record for Service was not found")
		})

		It("should create DNS records for an Ingress with external-dns annotation", func() {
			ctx := context.Background()

			By("Preparing test domain")
			host := "ingress"
			ingressDomain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Ingress with external-dns annotation")
			err := kubeClient.CreateTestIngress(ctx, testNamespace, testName, ingressDomain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test Ingress")

			By("Waiting for external-dns to process and create DNS record")
			// Wait for external-dns to process and create DNS record (max wait 2 minutes)
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for DNS record")
			Expect(success).To(BeTrue(), "DNS record was not created within timeout")

			By("Verifying DNS record is correctly created")
			records, err := pzClient.ListRecords(ctx, testZoneID)
			Expect(err).NotTo(HaveOccurred(), "Failed to list DNS records")

			found := false
			for _, record := range records {
				if *record.Host == host && *record.Type == "A" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "DNS record for Ingress was not found")
		})

		It("should create wildcard DNS records correctly", func() {
			ctx := context.Background()

			By("Preparing test domain")
			host := "*.wildcard"
			wildcardDomain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Service with wildcard domain")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, wildcardDomain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create wildcard Service")

			By("Waiting for external-dns to process and create DNS record")
			// Wait for external-dns to process and create DNS record (max wait 2 minutes)
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for wildcard DNS record")
			Expect(success).To(BeTrue(), "Wildcard DNS record was not created within timeout")
		})
	})

	Describe("DNS record creation, update and deletion", func() {
		It("should support creating, updating and deleting DNS records", func() {
			ctx := context.Background()

			By("Preparing test domain")
			host := "update-test"
			domain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Service with external-dns annotation")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, domain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test Service")

			By("Waiting for external-dns to process and create DNS record")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for DNS record")
			Expect(success).To(BeTrue(), "DNS record was not created within timeout")

			By("Getting created DNS record")
			record, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get DNS record")
			Expect(record).NotTo(BeNil(), "DNS record should not be nil")

			By("Deleting Kubernetes Service to trigger DNS record deletion")
			err = kubeClient.DeleteTestResources(ctx, testNamespace, testName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete test resources")

			By("Waiting for DNS record to be deleted")
			success, err = pzClient.WaitForRecordDeleted(ctx, testZoneID, host, "A", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for DNS record deletion")
			Expect(success).To(BeTrue(), "DNS record was not deleted within timeout")
		})
	})

	// Root record test case, privatezone uses @ as host value
	Describe("Root record tests", func() {
		It("should create, update and delete root domain DNS records correctly", func() {
			ctx := context.Background()

			By("Preparing root record")
			// Root domain is testDomain itself, host value is ""
			host := "@"
			domain := testDomain // Use testDomain directly as root domain

			By("Creating Service with external-dns annotation for root record")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, domain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create root domain Service")

			By("Waiting for external-dns to process and create root domain DNS record")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for root domain DNS record")
			Expect(success).To(BeTrue(), "Root domain DNS record was not created within timeout")

			By("Verifying root domain DNS record is correctly created")
			record, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get root domain DNS record")
			Expect(record).NotTo(BeNil(), "Root domain DNS record should not be nil")
			Expect(*record.Host).To(Equal("@"), "Root domain host should be @")

			By("Updating TTL of root domain DNS record")
			updatedTTL := int32(123) // Special value to avoid conflict with default
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, domain, fmt.Sprintf("%d", updatedTTL), "")
			Expect(err).NotTo(HaveOccurred(), "Failed to update root domain Service TTL annotation")

			By("Waiting for root domain DNS record TTL update to complete")
			success, err = kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, host, "A", "", updatedTTL, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for root domain DNS record TTL update")
			Expect(success).To(BeTrue(), "Root domain DNS record TTL was not updated within timeout")

			By("Verifying root domain DNS record TTL has been updated")
			updatedRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get updated root domain DNS record")
			Expect(*updatedRecord.TTL).To(Equal(updatedTTL), "Root domain DNS record TTL was not updated correctly")

			By("Updating value of root domain DNS record")
			newTargetIP := "192.168.100.100"
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, "", "", newTargetIP)
			Expect(err).NotTo(HaveOccurred(), "Failed to update root domain Service target annotation")

			By("Waiting for root domain DNS record value update to complete")
			success, err = kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, host, "A", newTargetIP, 0, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for root domain DNS record value update")
			Expect(success).To(BeTrue(), "Root domain DNS record value was not updated within timeout")

			By("Verifying root domain DNS record value has been updated")
			valueUpdatedRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get root domain DNS record with updated value")
			Expect(*valueUpdatedRecord.Value).To(Equal(newTargetIP), "Root domain DNS record value was not updated correctly")

			By("Deleting Kubernetes Service to trigger root domain DNS record deletion")
			err = kubeClient.DeleteTestResources(ctx, testNamespace, testName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete root domain test resources")

			By("Waiting for root domain DNS record to be deleted")
			success, err = pzClient.WaitForRecordDeleted(ctx, testZoneID, host, "A", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for root domain DNS record deletion")
			Expect(success).To(BeTrue(), "Root domain DNS record was not deleted within timeout")
		})
	})

	// Keep separate as it tests CNAME specific functionality
	Describe("CNAME record tests", func() {
		It("should support creating and verifying CNAME records through ExternalName Service", func() {
			ctx := context.Background()

			By("Preparing CNAME record information")
			host := "cname-test"
			domain := fmt.Sprintf("%s.%s", host, testDomain)
			externalName := "target.example.com"

			By("Creating ExternalName type Service to control CNAME record")
			err := kubeClient.CreateTestExternalNameService(ctx, testNamespace, testName, domain, externalName)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ExternalName Service")

			By("Waiting for CNAME record creation to complete")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for CNAME record")
			Expect(success).To(BeTrue(), "CNAME record was not created within timeout")

			By("Verifying CNAME record is correctly created")
			record, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "CNAME")
			Expect(err).NotTo(HaveOccurred(), "Failed to get CNAME record")
			Expect(*record.Type).To(Equal("CNAME"), "Record type should be CNAME")
			Expect(strings.TrimSuffix(*record.Value, ".")).To(Equal(externalName), "CNAME value is incorrect")

			By("Updating ExternalName Service's externalName")
			newExternalName := "newtarget.example.com"
			err = kubeClient.UpdateTestExternalNameService(ctx, testNamespace, testName, domain, newExternalName)
			Expect(err).NotTo(HaveOccurred(), "Failed to update ExternalName Service")

			By("Waiting for CNAME record update to complete")
			success, err = kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, host, "CNAME", newExternalName, 0, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for CNAME record update")
			Expect(success).To(BeTrue(), "CNAME record was not updated within timeout")

			By("Verifying CNAME record is correctly updated")
			updatedRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "CNAME")
			Expect(err).NotTo(HaveOccurred(), "Failed to get updated CNAME record")
			Expect(strings.TrimSuffix(*updatedRecord.Value, ".")).To(Equal(newExternalName), "CNAME value was not updated correctly")

		})
	})

	// Merge update-related tests
	Describe("DNS record update tests", func() {
		It("should update DNS record TTL when Service TTL annotation is updated", func() {
			ctx := context.Background()

			By("Preparing test domain")
			host := "service-ttl-test"
			domain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Service with external-dns annotation but without explicit TTL")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, domain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create service")

			By("Waiting for DNS record creation to complete")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for DNS record")
			Expect(success).To(BeTrue(), "DNS record was not created within timeout")

			By("Verifying initial DNS record TTL value")
			record, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get DNS record")
			initialTTL := *record.TTL
			Expect(initialTTL).To(BeNumerically(">", 0), "Initial TTL should be greater than 0")

			By("Updating Service's TTL annotation")
			updatedTTL := int32(123) // Special value to avoid conflict with default
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, domain, fmt.Sprintf("%d", updatedTTL), "")
			Expect(err).NotTo(HaveOccurred(), "Failed to update service TTL annotation")

			By("Waiting for DNS record TTL update to complete")
			success, err = kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, host, "A", "", updatedTTL, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for DNS record TTL update")
			Expect(success).To(BeTrue(), "DNS record TTL was not updated within timeout")

			By("Verifying DNS record TTL has been updated")
			updatedRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get updated DNS record")
			Expect(*updatedRecord.TTL).To(Equal(updatedTTL), "DNS record TTL was not updated correctly")
			Expect(*updatedRecord.TTL).NotTo(Equal(initialTTL), "DNS record TTL should be different from initial value")
		})

		It("should update DNS record TTL and hostname when Ingress annotations are updated", func() {
			ctx := context.Background()

			By("Preparing initial test domain")
			initialHost := "ingress-update-test"
			initialDomain := fmt.Sprintf("%s.%s", initialHost, testDomain)

			By("Creating initial Ingress with external-dns annotation")
			err := kubeClient.CreateTestIngress(ctx, testNamespace, testName, initialDomain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create initial test Ingress")

			By("Waiting for external-dns to process and create initial DNS record")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, initialHost, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for initial DNS record")
			Expect(success).To(BeTrue(), "Initial DNS record was not created within timeout")

			By("Getting TTL of initially created DNS record")
			initialRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, initialHost, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get initial DNS record")
			initialTTL := *initialRecord.TTL
			Expect(initialTTL).NotTo(Equal(int32(123)), "Initial TTL should not be 123")

			By("Updating Ingress's domain and TTL annotations")
			newHost := "updated-ingress"
			newDomain := fmt.Sprintf("%s.%s", newHost, testDomain)
			newTTL := "123" // Special value to avoid conflict with default
			err = kubeClient.UpdateTestIngress(ctx, testNamespace, testName, initialDomain, newDomain, newTTL, "")
			Expect(err).NotTo(HaveOccurred(), "Failed to update test Ingress")

			By("Waiting for external-dns to process and update DNS record")
			updated, err := kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, newHost, "A", "", int32(123), 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for DNS record TTL update")
			Expect(updated).To(BeTrue(), "DNS record was not updated within timeout period")

			By("Verifying DNS record for new domain has been created")
			newRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, newHost, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get updated DNS record")
			Expect(*newRecord.TTL).To(Equal(int32(123)), "TTL was not updated correctly via Ingress")

			By("Verifying DNS record for old domain has been deleted")
			oldRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, initialHost, "A")
			if err == nil {
				// If no error, the record still exists, verify if it's nil
				Expect(oldRecord).To(BeNil(), "Old DNS record should be deleted after Ingress update")
			}
		})

		It("should update DNS record value when Service target annotation is updated", func() {
			ctx := context.Background()

			By("Preparing test domain")
			host := "service-target-test"
			domain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Service with external-dns annotation")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, domain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create initial test Service")

			By("Waiting for external-dns to process and create initial DNS record")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for initial DNS record")
			Expect(success).To(BeTrue(), "Initial DNS record was not created within timeout")

			By("Getting value of initially created DNS record")
			initialRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get initial DNS record")
			initialValue := *initialRecord.Value
			Expect(initialValue).To(Not(Equal("")), "Initial DNS record value should not be empty")

			By("Setting Service's target annotation to specify new IP address")
			newTargetIP := "192.168.100.100"
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, "", "", newTargetIP)
			Expect(err).NotTo(HaveOccurred(), "Failed to update Service target annotation")

			By("Waiting for external-dns to process and update DNS record value")
			// Wait long enough for external-dns to detect the change and update the record
			updated, err := kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, host, "A", newTargetIP, 0, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for DNS record value update")
			Expect(updated).To(BeTrue(), "DNS record value was not updated within timeout period")

			By("Verifying DNS record value has been updated")
			updatedRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get updated DNS record")
			Expect(*updatedRecord.Value).To(Equal(newTargetIP), "DNS record value was not updated correctly via target annotation")

			By("Updating target annotation again with different IP address")
			anotherTargetIP := "192.168.100.200"
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, "", "", anotherTargetIP)
			Expect(err).NotTo(HaveOccurred(), "Failed to update Service target annotation again")

			By("Waiting for second record value update")
			updated, err = kubeClient.WaitForDNSRecordUpdate(ctx, pzClient, testZoneID, host, "A", anotherTargetIP, 0, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for second DNS record value update")
			Expect(updated).To(BeTrue(), "DNS record value was not updated in second update within timeout period")

			By("Verifying DNS record value has been updated again")
			secondUpdatedRecord, err := pzClient.GetRecordByHostAndType(ctx, testZoneID, host, "A")
			Expect(err).NotTo(HaveOccurred(), "Failed to get second updated DNS record")
			Expect(*secondUpdatedRecord.Value).To(Equal(anotherTargetIP), "DNS record value was not updated correctly in second update")
		})

		It("should update DNS records correctly when multiple targets are specified", func() {
			ctx := context.Background()

			By("Preparing test domain")
			host := "service-multitarget-test"
			domain := fmt.Sprintf("%s.%s", host, testDomain)

			By("Creating Service with external-dns annotation")
			err := kubeClient.CreateTestService(ctx, testNamespace, testName, domain)
			Expect(err).NotTo(HaveOccurred(), "Failed to create initial test Service")

			By("Waiting for external-dns to process and create initial DNS record")
			success, err := kubeClient.WaitForDNSRecord(ctx, pzClient, testZoneID, host, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Error waiting for initial DNS record")
			Expect(success).To(BeTrue(), "Initial DNS record was not created within timeout")

			By("Setting Service's target annotation to multiple IP addresses")
			multiTargets := "192.168.1.10,192.168.1.11,192.168.1.12"
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, "", "", multiTargets)
			Expect(err).NotTo(HaveOccurred(), "Failed to update Service with multiple targets")

			By("Waiting for external-dns to process and update DNS record")
			time.Sleep(1 * time.Minute)

			By("Getting all DNS records matching the hostname")
			allRecords, err := pzClient.ListRecords(ctx, testZoneID)
			Expect(err).NotTo(HaveOccurred(), "Failed to list all DNS records")

			var targetRecords []interface{}
			expectedIPs := strings.Split(multiTargets, ",")
			foundIPs := make(map[string]bool)

			By("Verifying all specified IP addresses have corresponding DNS records created")
			for _, record := range allRecords {
				if *record.Host == host && *record.Type == "A" {
					targetRecords = append(targetRecords, record)
					foundIPs[*record.Value] = true
				}
			}

			fmt.Printf("Found %d IP addresses in DNS records, expected %d\n", len(targetRecords), len(expectedIPs))
			// Verify record count matches specified IP count
			Expect(len(targetRecords)).To(Equal(len(expectedIPs)),
				fmt.Sprintf("Found %d IP addresses in DNS records, expected %d\n", len(targetRecords), len(expectedIPs)))

			// Verify each specified IP address has a corresponding DNS record
			for _, ip := range expectedIPs {
				Expect(foundIPs[ip]).To(BeTrue(),
					fmt.Sprintf("DNS record for IP %s was not found", ip))
			}

			By("Updating target annotation with different multiple IP addresses")
			updatedMultiTargets := "192.168.1.20,192.168.1.21"
			err = kubeClient.UpdateTestService(ctx, testNamespace, testName, "", "", updatedMultiTargets)
			Expect(err).NotTo(HaveOccurred(), "Failed to update Service with new multiple targets")

			By("Waiting for second multi-targets update")
			time.Sleep(1 * time.Minute)

			By("Getting all updated DNS records")
			updatedAllRecords, err := pzClient.ListRecords(ctx, testZoneID)
			Expect(err).NotTo(HaveOccurred(), "Failed to list updated DNS records")

			var updatedTargetRecords []interface{}
			newExpectedIPs := strings.Split(updatedMultiTargets, ",")
			newFoundIPs := make(map[string]bool)

			By("Verifying DNS records have been updated to new multiple IP addresses")
			for _, record := range updatedAllRecords {
				if *record.Host == host && *record.Type == "A" {
					updatedTargetRecords = append(updatedTargetRecords, record)
					newFoundIPs[*record.Value] = true
				}
			}

			// Verify updated record count matches newly specified IP count
			Expect(len(updatedTargetRecords)).To(Equal(len(newExpectedIPs)),
				fmt.Sprintf("Found %d IP addresses in DNS records, expected %d\n", len(updatedTargetRecords), len(newExpectedIPs)))

			// Verify each newly specified IP address has a corresponding DNS record
			for _, ip := range newExpectedIPs {
				Expect(newFoundIPs[ip]).To(BeTrue(),
					fmt.Sprintf("Updated DNS record for IP %s was not found", ip))
			}

			// Verify old IP address records have been deleted
			for _, oldIp := range expectedIPs {
				Expect(newFoundIPs[oldIp]).To(BeFalse(),
					fmt.Sprintf("Old DNS record for IP %s should be deleted", oldIp))
			}
		})
	})
})
