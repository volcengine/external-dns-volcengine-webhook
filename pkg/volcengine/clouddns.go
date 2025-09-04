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
	"os"
	"volcengine-provider/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volc-sdk-golang/service/dns"
)

type cloudZoneAPI interface {
}

type czWrapper struct {
	client *dns.Client
}

// The newCzWrapper creates a new CloudZone wrapper.
func newCzWrapper(region, endpoint string) *czWrapper {
	volcCaller := dns.NewVolcCaller()
	volcCaller.Volc.SetHost(endpoint)
	volcCaller.Volc.SetScheme("https")
	volcCaller.Volc.ServiceInfo.Credentials.Region = region
	volcCaller.Volc.SetAccessKey(os.Getenv("VOLC_ACCESSKEY"))
	volcCaller.Volc.SetSecretKey(os.Getenv("VOLC_SECRETKEY"))
	logrus.Debugf("cloudZone: Region: %s, endpoint: %s, ak:%s, sk:%s", volcCaller.Volc.ServiceInfo.Credentials.Region, volcCaller.Volc.ServiceInfo.Host, utils.MaskSecret(os.Getenv("VOLC_ACCESSKEY")), utils.MaskSecret(os.Getenv("VOLC_SECRETKEY")))
	return &czWrapper{
		client: dns.NewClient(volcCaller),
	}
}
