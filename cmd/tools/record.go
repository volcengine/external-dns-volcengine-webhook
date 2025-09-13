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

package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"

	"volcengine-provider/pkg/volcengine"
)

var (
	RecordCmd = &cobra.Command{
		Use:   "record",
		Short: "Add/Delete/List records",
	}
	recordAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add record",
		Run: func(cmd *cobra.Command, args []string) {
			recordAddHandler()
		},
	}
	recordDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete record",
		Run: func(cmd *cobra.Command, args []string) {
			recordDelHandler()
		},
	}
	recordListCmd = &cobra.Command{
		Use:   "list",
		Short: "List record",
		Run: func(cmd *cobra.Command, args []string) {
			recordListHandler()
		},
	}

	record string
	zone   int64
)

func init() {
	RecordCmd.PersistentFlags().Int64Var(&zone, "zone", 0, "zone id")
	recordAddCmd.PersistentFlags().StringVar(&record, "record", "", "record to add, like host#type#target")
	recordDeleteCmd.PersistentFlags().StringVar(&record, "record", "", "record to delete, like host#type#target")

	RecordCmd.AddCommand(recordAddCmd)
	RecordCmd.AddCommand(recordDeleteCmd)
	RecordCmd.AddCommand(recordListCmd)
}

func newPrivateZoneClient() (*volcengine.PrivateZoneWrapper, error) {
	accessKey := viper.GetString("access_key")
	secretKey := viper.GetString("secret_key")
	stsEndpoint := viper.GetString("sts_endpoint")
	oidcTokenFile := viper.GetString("oidc_token_file")
	roleTrn := viper.GetString("role_trn")
	var c *credentials.Credentials
	if accessKey != "" && secretKey != "" {
		log.Infof("Using static credentials with access_key=%s and secret_key=%s\n", volcengine.MaskSecret(accessKey), volcengine.MaskSecret(secretKey))
		c = credentials.NewStaticCredentials(accessKey, secretKey, "")
	} else if oidcTokenFile != "" && roleTrn != "" {
		log.Infof("Using oidc token file with oidcTokenFile=%s role_trn=%s \n", oidcTokenFile, roleTrn)
		p := credentials.NewOIDCCredentialsProviderFromEnv()
		p.OIDCTokenFilePath = oidcTokenFile
		p.RoleTrn = roleTrn
		p.Endpoint = stsEndpoint
		p.RoleTrn = roleTrn
		p.RoleSessionName = "external-dns"
		c = credentials.NewCredentials(p)
	} else {
		return nil, fmt.Errorf("aksk or oidc token file is required")
	}
	client, err := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("privatezone_endpoint"), c)
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		return nil, err
	}

	return client, nil
}

func recordListHandler() {
	client, err := newPrivateZoneClient()
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		os.Exit(1)
	}
	if zone != 0 {
		if err := listRecordByZid(client, zone); err != nil {
			log.Errorf("Failed to show record: %v", err)
			return
		}
	} else {
		if err := listRecordByVpc(client, viper.GetString("vpc")); err != nil {
			log.Errorf("Failed to show record: %v", err)
			return
		}
	}
}

func recordAddHandler() {
	client, err := newPrivateZoneClient()
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		os.Exit(1)
	}
	recordValue := strings.Split(record, "#")
	if len(recordValue) != 3 {
		log.Errorf("Invalid record value: %s", record)
		return
	}
	if err := addRecord(client, recordValue[0], recordValue[1], recordValue[2]); err != nil {
		log.Errorf("Add record error: %v", err)
		return
	}
}

func recordDelHandler() {
	client, err := newPrivateZoneClient()
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		os.Exit(1)
	}
	recordValue := strings.Split(record, "#")
	if len(recordValue) != 3 {
		log.Errorf("Invalid record value: %s", record)
		return
	}
	if err := delRecord(client, recordValue[0], recordValue[1], recordValue[2]); err != nil {
		log.Errorf("Delete record error: %v", err)
		return
	}
}

func addRecord(client *volcengine.PrivateZoneWrapper, host string, recordType string, target string) error {
	log.Debugf("add record: %s, type: %s, target: %s", host, recordType, target)
	err := client.CreatePrivateZoneRecord(context.Background(), zone, host, recordType, target, 0)
	if err != nil {
		log.Errorf("Failed to add record: %v", err)
		return err
	}
	return nil
}

func delRecord(client *volcengine.PrivateZoneWrapper, host string, recordType, target string) error {
	log.Debugf("del record: %s", host)
	err := client.DeletePrivateZoneRecord(context.Background(), zone, host, recordType, []string{target})
	if err != nil {
		log.Errorf("Failed to del record: %v", err)
		return err
	}
	return nil
}

func listRecordByZid(client *volcengine.PrivateZoneWrapper, zoneID int64) error {
	log.Debugf("list record: %d", zoneID)
	records, err := client.GetPrivateZoneRecords(context.Background(), zoneID)
	if err != nil {
		log.Errorf("Failed to show record: %v", err)
		return err
	}
	for _, r := range records {
		if r.Host != nil {
			log.Infof("id: %s, host: %s, type: %s, target: %s, ttl: %d", *r.RecordID, *r.Host, *r.Type, *r.Value, *r.TTL)
		}
	}
	return nil
}

func listRecordByVpc(client *volcengine.PrivateZoneWrapper, vpcID string) error {
	log.Debugf("list record: %s", vpcID)
	endpoints, err := client.ListRecordsByVPC(context.Background(), vpcID)
	if err != nil {
		log.Errorf("Failed to show record: %v", err)
		return err
	}
	for _, ep := range endpoints {
		fmt.Printf("dns:%s endpoints:%s\n", ep.DNSName, ep.Targets.String())
	}

	return nil
}
