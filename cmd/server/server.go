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

package server

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"volcengine-provider/pkg/volcengine"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

// Initialize the start command
var (
	StartCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the webhook server",
		Run: func(cmd *cobra.Command, args []string) {
			startServer()
		},
	}

	readTimeOut  int
	writeTimeOut int
)

type startConfig struct {
	Port            int
	AccessKey       string
	SecretKey       string
	VpcID           string
	RegionID        string
	PrivateZoneEP   string
	STSEndpoint     string
	OIDCTokenFile   string
	OIDCRoleTrn     string
	RoleTrn         string
	RoleSessionName string
	DurationSeconds int32
	DomainFilter    string
}

func init() {
	// Bind flags to the start command
	StartCmd.Flags().Int("port", 8888, "Port to listen on")
	StartCmd.Flags().IntVarP(&readTimeOut, "read_timeout", "", 60, "Read timeout in seconds")
	StartCmd.Flags().IntVarP(&writeTimeOut, "write_timeout", "", 60, "Write timeout in seconds")

	// Bind flags to Viper
	err := viper.BindPFlag("port", StartCmd.Flags().Lookup("port"))
	if err != nil {
		log.Fatalf("failed to bind flags: %v", err)
	}
}

func startServer() {
	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		log.Infof("No configuration file found: %v\n", err)
	}
	cfg := loadStartConfig(viper.GetViper())

	// Print debug logs if enabled
	log.Debugf("Starting server with configuration: port=%d, access_key=%s, secret_key=%s vpc=%s, endpoint=%s, region=%s, oidc_token_file=%s oidc_role_trn=%s role_trn=%s role_session_name=%s duration_seconds=%d\n",
		cfg.Port,
		volcengine.MaskSecret(cfg.AccessKey),
		volcengine.MaskSecret(cfg.SecretKey),
		cfg.VpcID,
		cfg.PrivateZoneEP,
		cfg.RegionID,
		cfg.OIDCTokenFile,
		cfg.OIDCRoleTrn,
		cfg.RoleTrn,
		cfg.RoleSessionName,
		cfg.DurationSeconds,
	)

	options, err := buildProviderOptions(cfg)
	if err != nil {
		panic(err)
	}

	provider, err := volcengine.NewVolcengineProvider(options)
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, // Normal termination signal
		syscall.SIGINT,  // Ctrl+C interrupt
		// syscall.SIGKILL cannot be caught (kernel-level signal)
	)
	defer stop()

	startedChan := make(chan struct{})
	go api.StartHTTPApi(
		provider, startedChan,
		time.Duration(readTimeOut)*time.Second,
		time.Duration(writeTimeOut)*time.Second,
		fmt.Sprintf("0.0.0.0:%d", cfg.Port),
	)

	// Wait for the HTTP server to start and then set the healthy and ready flags
	<-startedChan
	log.Infof("Listening on port %d...\n", cfg.Port)

	<-ctx.Done()
	log.Infof("Shutting down...\n")
}

func loadStartConfig(v *viper.Viper) startConfig {
	return startConfig{
		Port:            v.GetInt("port"),
		AccessKey:       v.GetString("access_key"),
		SecretKey:       v.GetString("secret_key"),
		VpcID:           v.GetString("vpc"),
		RegionID:        v.GetString("region"),
		PrivateZoneEP:   v.GetString("privatezone_endpoint"),
		STSEndpoint:     v.GetString("sts_endpoint"),
		OIDCTokenFile:   v.GetString("oidc_token_file"),
		OIDCRoleTrn:     v.GetString("oidc_role_trn"),
		RoleTrn:         v.GetString("role_trn"),
		RoleSessionName: v.GetString("role_session_name"),
		DurationSeconds: int32(v.GetInt("duration_seconds")),
		DomainFilter:    v.GetString("domain_filter"),
	}
}

func buildProviderOptions(cfg startConfig) ([]volcengine.Option, error) {
	options := []volcengine.Option{
		volcengine.WithPrivateZone(cfg.RegionID, cfg.VpcID),
		volcengine.WithPrivateZoneEndpoint(cfg.PrivateZoneEP),
	}

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		log.Infof("Using static credentials with access_key=%s and secret_key=%s\n", volcengine.MaskSecret(cfg.AccessKey), volcengine.MaskSecret(cfg.SecretKey))
		options = append(options, volcengine.WithStaticCredentials(cfg.AccessKey, cfg.SecretKey))
	} else if cfg.OIDCTokenFile != "" && cfg.OIDCRoleTrn != "" {
		log.Infof("Using oidc token file with oidcTokenFile=%s oidc_role_trn=%s\n", cfg.OIDCTokenFile, cfg.OIDCRoleTrn)
		options = append(options, volcengine.WithOIDCCredentials(cfg.STSEndpoint, cfg.OIDCRoleTrn, cfg.OIDCTokenFile))
	} else {
		return nil, fmt.Errorf("aksk or oidc token file is required")
	}

	if cfg.RoleTrn != "" {
		log.Infof("Using assume role with role_trn=%s role_session_name=%s duration_seconds=%d\n", cfg.RoleTrn, cfg.RoleSessionName, cfg.DurationSeconds)
		options = append(options, volcengine.WithAssumeRole(cfg.RegionID, cfg.STSEndpoint, cfg.RoleTrn, cfg.RoleSessionName, cfg.DurationSeconds))
	}

	if cfg.DomainFilter != "" {
		log.Infof("Using domain_filter=%s\n", cfg.DomainFilter)
		options = append(options, volcengine.WithDomainFilter(cfg.DomainFilter))
	}

	return options, nil
}
