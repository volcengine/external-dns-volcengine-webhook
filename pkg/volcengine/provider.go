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

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const (
	defaultEndpoint    = "open.volcengineapi.com"
	defaultStsEndpoint = "sts.volcengineapi.com"
)

// Provider is a provider for Volcengine.
type Provider struct {
	provider.BaseProvider

	// private zone
	vpcID       string
	privateZone bool
	pzClient    privateZoneAPI
}

type Option func(*Config)

// Config is the configuration for the Volcengine provider.
type Config struct {
	RegionID    string
	Credentials *credentials.Credentials

	// private zone
	PrivateZone         bool
	VpcId               string
	PrivateZoneEndpoint string
}

func defaultConfig() *Config {
	return &Config{
		PrivateZoneEndpoint: defaultEndpoint,
	}
}

// NewVolcengineProvider creates a new Volcengine provider.
func NewVolcengineProvider(options []Option) (*Provider, error) {
	var err error
	c := defaultConfig()
	for _, option := range options {
		option(c)
	}
	p := &Provider{
		vpcID:       c.VpcId,
		privateZone: c.PrivateZone,
	}
	// private zone, only support private zone now
	if p.privateZone {
		p.pzClient, err = NewPrivateZoneWrapper(c.RegionID, c.PrivateZoneEndpoint, c.Credentials)
		if err != nil {
			return nil, fmt.Errorf("failed to create private zone wrapper: %v", err)
		}
	}
	return p, nil
}

// Records returns the list of endpoints for the provider.
// Implementation for provider.Provider
func (p *Provider) Records(ctx context.Context) (endpoints []*endpoint.Endpoint, err error) {
	logrus.Infof("List Volcengine records, vpc: %s, privatezone:%t", p.vpcID, p.privateZone)
	if p.privateZone {
		return p.pzClient.ListRecordsByVPC(ctx, p.vpcID)
	}
	return endpoints, err
}

// ApplyChanges applies the given changes to the provider.
// Implementation for provider.Provider
func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if changes == nil {
		// No op skip
		return nil
	}
	if p.privateZone {
		return p.applyChangesForPrivateZone(ctx, changes)
	}
	return nil
}

func (p *Provider) applyChangesForPrivateZone(ctx context.Context, changes *plan.Changes) error {
	logrus.Infof("ApplyChanges to Volcengine Private Zone: %++v", *changes)

	// step1: get all private zones bind to vpc
	vpcZones, err := p.pzClient.ListPrivateZones(ctx, p.vpcID)
	if err != nil {
		return err
	}
	zoneNameIDMapper := provider.ZoneIDName{}
	for _, zoneinfo := range vpcZones {
		zid := *zoneinfo.ZID
		zoneNameIDMapper[strconv.FormatInt(int64(zid), 10)] = *zoneinfo.ZoneName
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

	// TODO support update records sometime avoid DNS return NXDOMAIN during update
	// if len(toUpdate) > 0 {
	// 	if err := p.updatePrivateZoneRecords(ctx, zoneNameIDMapper, toUpdate); err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func (p *Provider) createPrivateZoneRecords(ctx context.Context, zones provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
	if len(endpoints) == 0 {
		logrus.Info("No endpoints to create")
		return nil
	}

	endpointsByZone := separateCreateChange(zones, endpoints)
	recordsMap := make(map[int64][]*privatezone.RecordForBatchCreateRecordInput)
	for zid, ep := range endpointsByZone {
		zidInt, err := strconv.ParseInt(zid, 10, 64)
		if err != nil {
			logrus.Errorf("Failed to parse zid: %s", zid)
			return err
		}
		recordsMap[zidInt] = make([]*privatezone.RecordForBatchCreateRecordInput, 0)

		for _, record := range ep {
			for _, target := range record.Targets {
				host, domain := splitDNSName(record.DNSName, zones[zid])
				if domain == "" {
					logrus.Errorf("Failed to parse domain: %s, zoneId: %d, zoneName: %s", record.DNSName, zidInt, zones[zid])
					continue
				}
				value := target // Create a local variable copy
				if record.RecordType == "TXT" {
					value = escapeTXTRecordValue(value)
					logrus.Infof("Escape txt record for zone with value (%s), host: %s, zid: %d", value, host, zidInt)
				}
				var ttl *int32
				if record.RecordTTL > 0 {
					ttlInt32 := int32(record.RecordTTL)
					ttl = &ttlInt32
				}
				recordsMap[zidInt] = append(recordsMap[zidInt], &privatezone.RecordForBatchCreateRecordInput{
					Host:   &host,
					Type:   &record.RecordType,
					Value:  &value, // Use the address of the local variable
					TTL:    ttl,
					Remark: volcengine.String(defaultRecordRemark),
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

func (p *Provider) deletePrivateZoneRecords(ctx context.Context, zoneMap provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
	deletesByZone := make(map[string][]*endpoint.Endpoint, len(zoneMap))
	for _, z := range zoneMap {
		deletesByZone[z] = make([]*endpoint.Endpoint, 0)
	}
	for _, ep := range endpoints {
		// match longest zone name, private zone use longest zone name override short zone name
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
		for _, ep := range deletes {
			zoneName := zoneMap[zone]
			host, domain := splitDNSName(ep.DNSName, zoneName)
			logrus.Debugf("Deleting DNS record: '%s' type: '%s', zoneId: %s, zoneName: %s, host: %s, domain: %s", ep.DNSName, ep.RecordType, zone, zoneName, host, domain)
			if err := p.pzClient.DeletePrivateZoneRecord(ctx, zidInt, host, ep.RecordType, ep.Targets); err != nil {
				logrus.Errorf("Failed to delete private zone record: %s", err)
				return err
			}
		}
	}
	return nil
}
