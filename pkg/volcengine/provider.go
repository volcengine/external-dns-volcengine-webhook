package volcengine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/volcengine/volc-sdk-golang/service/privatezone"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

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

type VolcengineConfig struct {
	RegionID        string    `json:"regionId" yaml:"regionId"`
	AccessKeyID     string    `json:"accessKeyId" yaml:"accessKeyId"`
	AccessKeySecret string    `json:"accessKeySecret" yaml:"accessKeySecret"`
	PrivateZone     bool      `json:"privateZone" yaml:"privateZone"`
	VPCID           string    `json:"vpcId" yaml:"vpcId"`
	ExpireTime      time.Time `json:"-" yaml:"-"`
}

func NewVolcengineProvider(domainFilter endpoint.DomainFilter, zoneIDFilter provider.ZoneIDFilter, config VolcengineConfig) (*VolcengineProvider, error) {
	pzClient := NewPrivateZoneWrapper()
	czClient := NewCzWrapper()
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
	if p.privateZone {
		return Records(ctx, p.pzClient, p.vpcID)
	}
	return endpoints, err
}

func Records(ctx context.Context, pz privateZoneAPI, vpc string) (endpoints []*endpoint.Endpoint, err error) {
	log.Debugf("Retrieving Volcengine Private Zone records")

	// step 1: get all private zones
	zones, err := pz.GetPrivateZones(ctx)
	if err != nil {
		log.Errorf("Failed to list volcengine privatezones: %v", err)
		return nil, err
	}

	vpcZones := []privatezone.TopPrivateZoneResponse{}
	for _, zone := range zones {
		// filter out private zones
		zoneInfo, err := pz.GetPrivateZoneInfo(ctx, *zone.ZID)
		if err != nil {
			log.Errorf("Failed to get privatezone infos: %v", err)
			return nil, err
		}

		if vpc == "" {
			vpcZones = append(vpcZones, zone)
			continue
		}

		for _, bindVPC := range zoneInfo.BindVPCs {
			if *bindVPC.ID == vpc {
				log.Debugf("get matched zone: %s, vpc: %s", *zoneInfo.ZoneName, *bindVPC.ID)
				vpcZones = append(vpcZones, zone)
			}
		}
	}

	// step 2: get all record with zone
	for _, zone := range vpcZones {
		zid := strconv.FormatInt(*zone.ZID, 10)
		records, err := pz.GetPrivateZoneRecords(ctx, zid)
		if err != nil {
			log.Errorf("Failed to get privatezone records: %v", err)
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
	if changes == nil || len(changes.Create)+len(changes.Delete)+len(changes.UpdateNew) == 0 {
		// No op
		return nil
	}
	if p.privateZone {
		return p.applyChangesForPrivateZone(ctx, changes)
	}
	return nil
}

func (p *VolcengineProvider) applyChangesForPrivateZone(ctx context.Context, changes *plan.Changes) error {
	log.Infof("ApplyChanges to Volcengine Private Zone: %++v", *changes)

	// step1: get all private zones
	zones, err := p.pzClient.GetPrivateZones(ctx)
	if err != nil {
		return err
	}
	zoneNameIDMapper := provider.ZoneIDName{}
	for _, zoneinfo := range zones {
		zid := *zoneinfo.ZID
		zoneNameIDMapper[strconv.FormatInt(zid, 10)] = *zoneinfo.ZoneName
	}

	// step2: handle create record
	if err := p.createPrivateZoneRecords(ctx, zoneNameIDMapper, changes.Create); err != nil {
		return err
	}

	// step3: handle delete record
	if err := p.deletePrivateZoneRecords(ctx, zoneNameIDMapper, changes.Delete); err != nil {
		return err
	}

	// step4: handle update record
	if err := p.updatePrivateZoneRecords(ctx, zoneNameIDMapper, changes.UpdateNew); err != nil {
		return err
	}

	return nil
}

func (p *VolcengineProvider) createPrivateZoneRecords(ctx context.Context, zones provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
	if len(endpoints) == 0 {
		log.Info("No endpoints to create")
		return nil
	}

	endpointsByZone := separateCreateChange(zones, endpoints)
	for zid, ep := range endpointsByZone {
		zidInt, err := strconv.ParseInt(zid, 10, 64)
		if err != nil {
			log.Errorf("Failed to parse zid: %s", zid)
			return err
		}
		for _, record := range ep {
			for _, target := range record.Targets {
				if err := p.pzClient.CreatePrivateZoneRecord(ctx, zidInt, removeZonename(record.DNSName, zones[zid]), record.RecordType, target); err != nil {
					log.Errorf("Failed to create private zone record: %s", err)
					return err
				}
			}
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
		zone, _ := zoneMap.FindZone(ep.DNSName)
		if zone != "" {
			createsByZone[zone] = append(createsByZone[zone], ep)
			continue
		}
		log.Debugf("Skipping DNS creation of endpoint: '%s' type: '%s', it does not match against Domain filters", ep.DNSName, ep.RecordType)
	}

	return createsByZone
}

func removeZonename(longDomain, shortDomain string) string {
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
		zone, _ := zoneMap.FindZone(ep.DNSName)
		if zone != "" {
			deletesByZone[zone] = append(deletesByZone[zone], ep)
			continue
		}
		log.Debugf("Skipping DNS deletion of endpoint: '%s' type: '%s', it does not match against Domain filters", ep.DNSName, ep.RecordType)
	}
	for zone, deletes := range deletesByZone {
		if len(deletes) == 0 {
			continue
		}
		zidInt, err := strconv.ParseInt(zone, 10, 64)
		if err != nil {
			log.Errorf("Failed to parse zid: %s", zone)
			return err
		}
		for _, record := range deletes {
			if err := p.pzClient.DeletePrivateZoneRecord(ctx, zidInt, record.DNSName); err != nil {
				log.Errorf("Failed to delete private zone record: %s", err)
				return err
			}
		}
	}
	return nil
}

func (p *VolcengineProvider) updatePrivateZoneRecords(ctx context.Context, zones provider.ZoneIDName, endpoints []*endpoint.Endpoint) error {
	// update record must use record id
	endpointsByZone := separateCreateChange(zones, endpoints)
	for zone, updates := range endpointsByZone {
		zidInt, err := strconv.ParseInt(zone, 10, 64)
		if err != nil {
			log.Errorf("Failed to parse zid: %s", zone)
			return err
		}
		for _, record := range updates {
			if err := p.pzClient.UpdatePrivateZoneRecord(ctx, zidInt, record.DNSName, record.Targets); err != nil {
				log.Errorf("Failed to update private zone record: %s", err)
				return err
			}
		}
	}
	return nil
}
