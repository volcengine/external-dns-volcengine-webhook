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

package volcengine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volc-sdk-golang/service/privatezone"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// VolcengineProvider is a provider for Volcengine.
type VolcengineProvider struct {
	provider.BaseProvider
	domainFilter   endpoint.DomainFilter
	zoneIDFilter   provider.ZoneIDFilter // Private Zone only
	MaxChangeCount int
	vpcID          string // Private Zone only
	privateZone    bool
	pzClient       privateZoneAPI
	czClient       cloudZoneAPI
}

// VolcengineConfig is the configuration for the Volcengine provider.
type VolcengineConfig struct {
	RegionID        string    `json:"regionId" yaml:"regionId"`
	AccessKeyID     string    `json:"accessKeyId" yaml:"accessKeyId"`
	AccessKeySecret string    `json:"accessKeySecret" yaml:"accessKeySecret"`
	PrivateZone     bool      `json:"privateZone" yaml:"privateZone"`
	VPCID           string    `json:"vpcId" yaml:"vpcId"`
	ExpireTime      time.Time `json:"-" yaml:"-"`
	Endpoint        string    `json:"endpoint" yaml:"endpoint"`
}

// NewVolcengineProvider creates a new Volcengine provider.
func NewVolcengineProvider(domainFilter endpoint.DomainFilter, zoneIDFilter provider.ZoneIDFilter, config VolcengineConfig) (*VolcengineProvider, error) {
	pzClient := NewPrivateZoneWrapper(config.RegionID, config.Endpoint)
	czClient := newCzWrapper(config.RegionID, config.Endpoint)
	return &VolcengineProvider{
		domainFilter: domainFilter,
		zoneIDFilter: zoneIDFilter,
		vpcID:        config.VPCID,
		privateZone:  config.PrivateZone,
		pzClient:     pzClient,
		czClient:     czClient,
	}, nil
}

func (p *VolcengineProvider) Records(ctx context.Context) (endpoints []*endpoint.Endpoint, err error) {

	logrus.Infof("List Volcengine records, vpc: %s, privatezone:%t", p.vpcID, p.privateZone)
	if p.privateZone {
		return Records(ctx, p.pzClient, p.vpcID)
	}
	return endpoints, err
}

func Records(ctx context.Context, pz privateZoneAPI, vpc string) (endpoints []*endpoint.Endpoint, err error) {
	logrus.Debugf("Retrieving Volcengine Private Zone records")

	// step 1: get all private zones bind to vpc
	vpcZones, err := pz.ListPrivateZones(ctx, vpc)
	if err != nil {
		logrus.Errorf("Failed to list volcengine privatezones: %v", err)
		return nil, err
	}

	// step 2: get all record with zone
	for _, zone := range vpcZones {
		zid := strconv.FormatInt(*zone.ZID, 10)
		records, err := pz.GetPrivateZoneRecords(ctx, zid)
		if err != nil {
			logrus.Errorf("Failed to get privatezone records: %v", err)
			return nil, err
		}

		if len(records) == 0 {
			continue
		}
		// step 3: convert record to endpoint
		for _, record := range records {
			// Domain: record.Host + "." + zoneInfo.ZoneName
			// Type:  record.Type
			// Target: record.Value
			// TTL: record.TTL
			endpoints = append(endpoints, &endpoint.Endpoint{
				DNSName:    fmt.Sprintf("%s.%s", *record.Host, *zone.ZoneName),
				RecordType: *record.Type,
				Targets:    []string{*record.Value},
				RecordTTL:  endpoint.TTL(*record.TTL),
			})
		}
	}

	return endpoints, nil
}

func (p *VolcengineProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if changes == nil {
		// No op
		return nil
	}
	if p.privateZone {
		return p.applyChangesForPrivateZone(ctx, changes)
	}
	return nil
}

func (p *VolcengineProvider) applyChangesForPrivateZone(ctx context.Context, changes *plan.Changes) error {
	logrus.Infof("ApplyChanges to Volcengine Private Zone: %++v", *changes)

	// step1: get all private zones bind to vpc
	vpcZones, err := p.pzClient.ListPrivateZones(ctx, p.vpcID)
	if err != nil {
		return err
	}
	zoneNameIDMapper := provider.ZoneIDName{}
	for _, zoneinfo := range vpcZones {
		zid := *zoneinfo.ZID
		zoneNameIDMapper[strconv.FormatInt(zid, 10)] = *zoneinfo.ZoneName
	}

	toCreate := make([]*endpoint.Endpoint, 0)
	toDelete := make([]*endpoint.Endpoint, 0)
	// toUpdate := make([]*endpoint.Endpoint, 0)

	toCreate = append(toCreate, changes.Create...)
	toDelete = append(toDelete, changes.Delete...)

	toCreate = append(toCreate, changes.UpdateNew...)
	toDelete = append(toDelete, changes.UpdateOld...)

	if len(toDelete) > 0 {
		if err := p.deletePrivateZoneRecords(ctx, zoneNameIDMapper, toDelete); err != nil {
			return err
		}
	}

	if len(toCreate) > 0 {
		if err := p.createPrivateZoneRecords(ctx, zoneNameIDMapper, toCreate); err != nil {
			return err
		}
	}

	// if len(toUpdate) > 0 {
	// 	if err := p.updatePrivateZoneRecords(ctx, zoneNameIDMapper, toUpdate); err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func (p *VolcengineProvider) createPrivateZoneRecords(ctx context.Context, zones provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
	if len(endpoints) == 0 {
		logrus.Info("No endpoints to create")
		return nil
	}

	endpointsByZone := separateCreateChange(zones, endpoints)
	recordsMap := make(map[int64][]privatezone.CRecord)
	for zid, ep := range endpointsByZone {
		zidInt, err := strconv.ParseInt(zid, 10, 64)
		if err != nil {
			logrus.Errorf("Failed to parse zid: %s", zid)
			return err
		}
		recordsMap[zidInt] = make([]privatezone.CRecord, 0)

		for _, record := range ep {
			for _, target := range record.Targets {
				value := target // 创建局部变量拷贝
				if record.RecordType == "TXT" {
					value = p.unescapeTXTRecordValue(value)
					logrus.Infof("Adding TXT record for zone with value (%s)", value)
				}
				host := removeZoneName(record.DNSName, zones[zid])
				var ttl *int64
				if record.RecordTTL > 0 {
					ttlInt64 := int64(record.RecordTTL)
					ttl = &ttlInt64
				}
				recordsMap[zidInt] = append(recordsMap[zidInt], privatezone.CRecord{
					Host:  &host,
					Type:  &record.RecordType,
					Value: &value, // 使用局部变量的地址
					TTL:   ttl,
				})
			}
		}
	}
	for zid, records := range recordsMap {
		if len(records) == 0 {
			continue
		}
		if err := p.pzClient.BatchCreatePrivateZoneRecord(ctx, zid, records); err != nil {
			logrus.Errorf("Failed to batch create private zone record: %s", err)
			return err
		}
	}
	return nil
}

// separateCreateChange separates a multi-zone change into a single change per zone.
func separateCreateChange(zoneMap provider.ZoneIDName, endpoints []*endpoint.Endpoint) map[string][]*endpoint.Endpoint {
	createsByZone := make(map[string][]*endpoint.Endpoint, len(zoneMap))
	for zid := range zoneMap {
		createsByZone[zid] = make([]*endpoint.Endpoint, 0)
	}
	for _, ep := range endpoints {
		zone, zoneName := zoneMap.FindZone(ep.DNSName)
		if zone != "" {
			createsByZone[zone] = append(createsByZone[zone], ep)
			logrus.Debugf("Adding DNS creation of endpoint: '%s' type: '%s', zoneId: %s, zoneName: %s", ep.DNSName, ep.RecordType, zone, zoneName)
			continue
		}
		logrus.Debugf("Skipping DNS creation of endpoint: '%s' type: '%s', it does not match against Domain filters", ep.DNSName, ep.RecordType)
	}

	return createsByZone
}

func removeZoneName(longDomain, shortDomain string) string {
	index := strings.Index(longDomain, shortDomain)
	if index != -1 {
		return strings.TrimSuffix(longDomain[:index], ".")
	}

	return longDomain
}

func (p *VolcengineProvider) deletePrivateZoneRecords(ctx context.Context, zoneMap provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
	deletesByZone := make(map[string][]*endpoint.Endpoint, len(zoneMap))
	for _, z := range zoneMap {
		deletesByZone[z] = make([]*endpoint.Endpoint, 0)
	}
	for _, ep := range endpoints {
		zone, zoneName := zoneMap.FindZone(ep.DNSName)
		if zone != "" {
			deletesByZone[zone] = append(deletesByZone[zone], ep)
			logrus.Debugf("Adding DNS deletion of endpoint: '%s' type: '%s', zoneId: %s, zoneName: %s", ep.DNSName, ep.RecordType, zone, zoneName)
			continue
		}
		logrus.Debugf("Skipping DNS deletion of endpoint: '%s' type: '%s', it does not match against Domain filters", ep.DNSName, ep.RecordType)
	}
	for zone, deletes := range deletesByZone {
		if len(deletes) == 0 {
			continue
		}
		zidInt, err := strconv.ParseInt(zone, 10, 64)
		if err != nil {
			logrus.Errorf("Failed to parse zid: %s", zone)
			return err
		}
		for _, record := range deletes {
			if err := p.pzClient.DeletePrivateZoneRecord(ctx, zidInt, record.DNSName); err != nil {
				logrus.Errorf("Failed to delete private zone record: %s", err)
				return err
			}
		}
	}
	return nil
}

func (p *VolcengineProvider) unescapeTXTRecordValue(value string) string {
	if strings.HasPrefix(value, "\"heritage=") {
		// remove \" in txt record value for volcengine privatezone
		return fmt.Sprintf("%s", strings.Replace(value, "\"", "", -1))
	}
	return value
}

// func (p *VolcengineProvider) updatePrivateZoneRecords(ctx context.Context, zones provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
// 	// update record must use record id
// 	endpointsByZone := separateCreateChange(zones, endpoints)
// 	for zone, updates := range endpointsByZone {
// 		zidInt, err := strconv.ParseInt(zone, 10, 64)
// 		if err != nil {
// 			log.Errorf("Failed to parse zid: %s", zone)
// 			return err
// 		}
// 		for _, record := range updates {
// 			if err := p.pzClient.UpdatePrivateZoneRecord(ctx, zidInt, record.DNSName, record.Targets); err != nil {
// 				log.Errorf("Failed to update private zone record: %s", err)
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }
