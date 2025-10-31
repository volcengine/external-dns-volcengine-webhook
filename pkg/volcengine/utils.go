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
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
)

// MaskSecret masks the secret with ****
func MaskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "********" + secret[len(secret)-4:]
}

// BatchForEach splits the items into batches and calls the function for each batch.
func BatchForEach[T any, R any](items []T, batchSize int, f func([]T) ([]R, error)) ([]R, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than 0")
	}
	n := len(items)
	if n == 0 {
		return []R{}, nil
	}
	var all []R
	for i := 0; i < n; i += batchSize {
		end := i + batchSize
		if end > n {
			end = n
		}
		part, err := f(items[i:end])
		if err != nil {
			return nil, err
		}
		all = append(all, part...)
	}

	return all, nil
}

// QueryAll is a generic pagination function: query is responsible for cloning, setting page number, and returning (data, total, err)
func QueryAll[T any](
	pageSize int,
	query func(int, int) ([]T, int, error),
) ([]T, error) {

	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize must be greater than 0")
	}
	var all []T
	pageNum := 1
	for {
		data, total, err := query(pageNum, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, data...)
		if pageNum*pageSize >= total {
			break
		}
		pageNum++
	}

	return all, nil
}

func escapeTXTRecordValue(value string) string {
	if strings.HasPrefix(value, "\"heritage=") {
		// remove \" in txt record value for volcengine privatezone
		return strings.ReplaceAll(value, "\"", "")
	}
	return value
}

func unescapeTXTRecordValue(value string) string {
	if strings.HasPrefix(value, "heritage=") {
		// add \" in txt record value for volcengine privatezone
		return fmt.Sprintf("\"%s\"", value)
	}
	return value
}

func getDNSName(host, domain string) string {
	if host == nullHostPrivateZone {
		return domain
	}
	return host + "." + domain
}

func splitDNSName(dnsName, zoneName string) (host string, domain string) {
	name := strings.TrimSuffix(dnsName, ".")
	if strings.HasSuffix(name, "."+zoneName) {
		host = name[0 : len(name)-len(zoneName)-1]
		domain = zoneName
	} else if name == zoneName {
		domain = zoneName
		host = ""
	}

	if host == "" {
		host = nullHostPrivateZone
	}
	return host, domain
}

func cleanCNAMEValue(value string) string {
	return strings.TrimSuffix(value, ".")
}

func completeCNAMEValue(value string) string {
	if !strings.HasSuffix(value, ".") {
		return value + "."
	}
	return value
}

type LoggerAdapter struct {
	*logrus.Entry
}

func NewLoggerAdapter(logger *logrus.Entry) *LoggerAdapter {
	return &LoggerAdapter{logger}
}

func (l *LoggerAdapter) Log(args ...interface{}) {
	l.Entry.Log(l.Logger.GetLevel(), args...)
}
