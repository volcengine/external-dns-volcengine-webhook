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

type recordCredentialConfig struct {
	AccessKey       string
	SecretKey       string
	Region          string
	STSEndpoint     string
	OIDCTokenFile   string
	OIDCRoleTrn     string
	AssumeRoleTrn   string
	RoleSessionName string
	DurationSeconds int32
}

func init() {
	RecordCmd.PersistentFlags().Int64Var(&zone, "zone", 0, "zone id")
	RecordCmd.PersistentFlags().String("sts_endpoint", "", "STS endpoint")
	RecordCmd.PersistentFlags().String("role_trn", "", "target role trn for AssumeRole")
	RecordCmd.PersistentFlags().String("role_session_name", "", "session name for AssumeRole")
	RecordCmd.PersistentFlags().Int32("duration_seconds", 0, "session duration seconds for AssumeRole")
	recordAddCmd.PersistentFlags().StringVar(&record, "record", "", "record to add, like host#type#target")
	recordDeleteCmd.PersistentFlags().StringVar(&record, "record", "", "record to delete, like host#type#target")

	mustBindRecordFlag("sts_endpoint")
	mustBindRecordFlag("role_trn")
	mustBindRecordFlag("role_session_name")
	mustBindRecordFlag("duration_seconds")

	RecordCmd.AddCommand(recordAddCmd)
	RecordCmd.AddCommand(recordDeleteCmd)
	RecordCmd.AddCommand(recordListCmd)
}

func mustBindRecordFlag(flagName string) {
	if err := viper.BindPFlag(flagName, RecordCmd.PersistentFlags().Lookup(flagName)); err != nil {
		log.Fatalf("failed to bind %s flag: %v", flagName, err)
	}
}

func newPrivateZoneClient() (*volcengine.PrivateZoneWrapper, error) {
	credentialConfig := recordCredentialConfigFromViper()
	c, err := buildRecordCredentials(credentialConfig)
	if err != nil {
		return nil, err
	}
	client, err := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("privatezone_endpoint"), c)
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		return nil, err
	}

	return client, nil
}

func recordCredentialConfigFromViper() recordCredentialConfig {
	assumeRoleTrn := viper.GetString("role_trn")
	oidcRoleTrn := viper.GetString("oidc_role_trn")
	if oidcRoleTrn == "" && viper.GetString("oidc_token_file") != "" && assumeRoleTrn != "" {
		log.Warn("`role_trn` is now reserved for target AssumeRole; falling back to it as legacy OIDC source role because `oidc_role_trn` is unset")
		oidcRoleTrn = assumeRoleTrn
		assumeRoleTrn = ""
	}

	return recordCredentialConfig{
		AccessKey:       viper.GetString("access_key"),
		SecretKey:       viper.GetString("secret_key"),
		Region:          viper.GetString("region"),
		STSEndpoint:     viper.GetString("sts_endpoint"),
		OIDCTokenFile:   viper.GetString("oidc_token_file"),
		OIDCRoleTrn:     oidcRoleTrn,
		AssumeRoleTrn:   assumeRoleTrn,
		RoleSessionName: viper.GetString("role_session_name"),
		DurationSeconds: viper.GetInt32("duration_seconds"),
	}
}

func buildRecordCredentials(config recordCredentialConfig) (*credentials.Credentials, error) {
	if config.AccessKey != "" && config.SecretKey != "" {
		log.Infof("Using static credentials with access_key=%s and secret_key=%s", volcengine.MaskSecret(config.AccessKey), volcengine.MaskSecret(config.SecretKey))
	} else if config.OIDCTokenFile != "" && config.OIDCRoleTrn != "" {
		log.Infof("Using oidc token file with oidcTokenFile=%s oidc_role_trn=%s", config.OIDCTokenFile, config.OIDCRoleTrn)
	}
	if config.AssumeRoleTrn != "" {
		log.Infof("Using AssumeRole target role_trn=%s role_session_name=%s duration_seconds=%d", config.AssumeRoleTrn, config.RoleSessionName, config.DurationSeconds)
	}

	return volcengine.NewCredentials(volcengine.CredentialOptions{
		AccessKey:     config.AccessKey,
		SecretKey:     config.SecretKey,
		STSEndpoint:   config.STSEndpoint,
		OIDCTokenFile: config.OIDCTokenFile,
		OIDCRoleTrn:   config.OIDCRoleTrn,
		AssumeRoleConfig: &volcengine.AssumeRoleOptions{
			Region:          config.Region,
			STSEndpoint:     config.STSEndpoint,
			RoleTrn:         config.AssumeRoleTrn,
			RoleSessionName: config.RoleSessionName,
			DurationSeconds: config.DurationSeconds,
		},
	})
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
	zones, err := client.ListPrivateZones(context.Background(), vpcID)
	if err != nil {
		log.Errorf("Failed to show record: %v", err)
		return err
	}
	for _, zone := range zones {
		if err = listRecordByZid(client, int64(*zone.ZID)); err != nil {
			log.Errorf("Failed to show record: %v", err)
			return err
		}
	}

	return nil
}
