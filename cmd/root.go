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

package cmd

import (
	"fmt"
	"os"

	"volcengine-provider/cmd/server"
	"volcengine-provider/cmd/tools"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logLevel = "info"

	rootCmd = &cobra.Command{
		Use: "volcengine-provider",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			lev, err := logrus.ParseLevel(logLevel)
			if err != nil {
				fmt.Printf("Error parsing log level: %s\n", err)
				lev = logrus.InfoLevel
			}
			logrus.SetLevel(lev)
			logrus.SetFormatter(&logrus.TextFormatter{})
			logrus.SetReportCaller(true)
		},
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", logLevel, "log level")
	rootCmd.AddCommand(server.StartCmd)
	rootCmd.AddCommand(tools.RecordCmd)

	// Bind environment variables
	viper.SetEnvPrefix("VOLCENGINE") // Prefix for environment variables
	viper.BindEnv("access_key")
	viper.BindEnv("secret_key")
	viper.BindEnv("vpc")
	viper.BindEnv("region")
	viper.BindEnv("privatezone_endpoint")
	viper.BindEnv("sts_endpoint")
	viper.BindEnv("oidc_token_file")
	viper.BindEnv("oidc_role_trn")
}
