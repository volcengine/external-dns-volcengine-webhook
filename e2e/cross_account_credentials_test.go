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
	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	sdkvolcengine "github.com/volcengine/volcengine-go-sdk/volcengine"
	sdkcredentials "github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"

	provider "volcengine-provider/pkg/volcengine"
)

var _ = Describe("Cross-account credential lifecycle", Label("cross-account"), func() {
	var (
		config     *TestConfig
		testZoneID int64
	)

	BeforeEach(func() {
		var err error
		config, err = LoadCrossAccountTestConfig()
		Expect(err).NotTo(HaveOccurred(), "Failed to load cross-account test config")

		if !config.CrossAccountEnabled {
			Skip("Cross-account e2e is disabled; set VOLCENGINE_E2E_CROSS_ACCOUNT=true to enable it")
		}
		if config.PrivateZoneID == "" {
			Skip("Cross-account credential lifecycle e2e requires PRIVATE_ZONE_ID")
		}
		if config.DurationSeconds <= 0 {
			Skip("Cross-account credential lifecycle e2e requires VOLCENGINE_DURATION_SECONDS to be configured")
		}

		testZoneID, err = parseZoneID(config.PrivateZoneID)
		Expect(err).NotTo(HaveOccurred(), "Failed to parse private zone ID")
	})

	It("should refresh cross-account credentials after they expire", func() {
		ctx := context.Background()

		By("Creating target account credentials backed by AssumeRole")
		creds, err := provider.NewCredentials(config.TargetCredentialOptions())
		Expect(err).NotTo(HaveOccurred(), "Failed to create target credentials")

		client, err := newPrivateZoneAPIWithCredentials(config, creds)
		Expect(err).NotTo(HaveOccurred(), "Failed to create PrivateZone client with target credentials")

		By("Using the initial temporary credentials to access the target account")
		_, err = client.ListRecordsWithContext(ctx, &privatezone.ListRecordsInput{ZID: &testZoneID})
		Expect(err).NotTo(HaveOccurred(), "Initial cross-account authorization should succeed")

		firstValue, err := creds.Get()
		Expect(err).NotTo(HaveOccurred(), "Failed to get initial temporary credentials")
		firstExpiresAt, err := creds.ExpiresAt()
		Expect(err).NotTo(HaveOccurred(), "Failed to get initial credential expiration")

		By("Forcing the temporary credentials to expire and triggering a refresh")
		time.Sleep(2 * time.Second)
		creds.Expire()

		_, err = client.ListRecordsWithContext(ctx, &privatezone.ListRecordsInput{ZID: &testZoneID})
		Expect(err).NotTo(HaveOccurred(), "Cross-account authorization should still succeed after refresh")

		secondValue, err := creds.Get()
		Expect(err).NotTo(HaveOccurred(), "Failed to get refreshed temporary credentials")
		secondExpiresAt, err := creds.ExpiresAt()
		Expect(err).NotTo(HaveOccurred(), "Failed to get refreshed credential expiration")

		By("Verifying the refreshed credentials are newly issued")
		refreshed := secondExpiresAt.After(firstExpiresAt) ||
			secondValue.SessionToken != firstValue.SessionToken ||
			secondValue.AccessKeyID != firstValue.AccessKeyID
		Expect(refreshed).To(BeTrue(), "Temporary credentials were not refreshed after expiration")
		Expect(secondExpiresAt).To(BeTemporally(">", time.Now().UTC()), "Refreshed credentials should still be valid")
	})
})

func parseZoneID(zoneID string) (int64, error) {
	parsed, err := strconv.ParseInt(zoneID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse zone id %q: %w", zoneID, err)
	}
	return parsed, nil
}

func newPrivateZoneAPIWithCredentials(config *TestConfig, creds *sdkcredentials.Credentials) (privatezone.PRIVATEZONEAPI, error) {
	volcConfig := sdkvolcengine.NewConfig().
		WithCredentials(creds).
		WithRegion(config.RegionID)

	sess, err := session.NewSession(volcConfig)
	if err != nil {
		return nil, fmt.Errorf("create privatezone session: %w", err)
	}

	return privatezone.New(sess), nil
}

func cleanupRecordsByHostAndType(ctx context.Context, pzClient *PrivateZoneClient, zoneID int64, host, recordType string) error {
	records, err := pzClient.ListRecords(ctx, zoneID)
	if err != nil {
		return err
	}

	for _, record := range records {
		if record.Host == nil || record.Type == nil || record.RecordID == nil {
			continue
		}
		if *record.Host == host && *record.Type == recordType {
			if err := pzClient.DeleteRecord(ctx, zoneID, *record.RecordID); err != nil {
				return err
			}
		}
	}

	return nil
}
