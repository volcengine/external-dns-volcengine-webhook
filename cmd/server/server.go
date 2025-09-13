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

func init() {
	// Bind flags to the start command
	StartCmd.Flags().Int("port", 8888, "Port to listen on")
	StartCmd.Flags().IntVarP(&readTimeOut, "read_timeout", "", 60, "Read timeout in seconds")
	StartCmd.Flags().IntVarP(&writeTimeOut, "write_timeout", "", 60, "Write timeout in seconds")

	// Bind flags to Viper
	viper.BindPFlag("port", StartCmd.Flags().Lookup("port"))
}

func startServer() {
	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		log.Infof("No configuration file found: %v\n", err)
	}
	// Read configuration values
	port := viper.GetInt("port")
	accessKey := viper.GetString("access_key")
	secretKey := viper.GetString("secret_key")
	vpcID := viper.GetString("vpc")
	regionID := viper.GetString("region")
	pvzEndpoint := viper.GetString("privatezone_endpoint")
	stsEndpoint := viper.GetString("sts_endpoint")
	oidcTokenFile := viper.GetString("oidc_token_file")
	oidcRoleTrn := viper.GetString("oidc_role_trn")

	// Print debug logs if enabled
	log.Debugf("Starting server with configuration: port=%d, access_key=%s, secret_key=%s vpc=%s, endpoint=%s, region=%s, oidc_token_file=%s oidc_role_trn=%s \n",
		port, volcengine.MaskSecret(accessKey), volcengine.MaskSecret(secretKey), vpcID, pvzEndpoint, regionID, oidcTokenFile, oidcRoleTrn)

	options := []volcengine.Option{
		volcengine.WithPrivateZone(regionID, vpcID),
		volcengine.WithPrivateZoneEndpoint(pvzEndpoint),
	}
	if accessKey != "" && secretKey != "" {
		log.Infof("Using static credentials with access_key=%s and secret_key=%s\n", volcengine.MaskSecret(accessKey), volcengine.MaskSecret(secretKey))
		options = append(options, volcengine.WithStaticCredentials(accessKey, secretKey))
	} else if oidcTokenFile != "" && oidcRoleTrn != "" {
		log.Infof("Using oidc token file with oidcTokenFile=%s oidc_role_trn=%s \n", oidcTokenFile, oidcRoleTrn)
		options = append(options, volcengine.WithOIDCCredentials(stsEndpoint, oidcRoleTrn, oidcTokenFile))
	} else {
		panic("aksk or oidc token file is required")
	}

	provider, err := volcengine.NewVolcengineProvider(options)
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, // 常规终止信号
		syscall.SIGINT,  // Ctrl+C 中断
		// syscall.SIGKILL 不可捕获（内核级信号）
	)
	defer stop()

	startedChan := make(chan struct{})
	go api.StartHTTPApi(
		provider, startedChan,
		time.Duration(readTimeOut)*time.Second,
		time.Duration(writeTimeOut)*time.Second,
		fmt.Sprintf("0.0.0.0:%d", port),
	)

	// Wait for the HTTP server to start and then set the healthy and ready flags
	<-startedChan
	log.Infof("Listening on port %d...\n", port)

	<-ctx.Done()
	log.Infof("Shutting down...\n")
}
