package volcengine

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/volcengine/volcengine-go-sdk/volcengine/response"
	"strings"
)

// MaskSecret masks the secret with ****
func MaskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

// BatchForEach splits the items into batches and calls the function for each batch.
func BatchForEach[T any, R any](items []T, batchSize int, f func([]T) ([]R, error)) ([]R, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than 0")
	}
	n := len(items)
	if n == 0 {
		return nil, nil
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

// QueryAll 泛型翻页：query 负责克隆+设页号+返回 (data,total,err)
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

// TraceOpenAPI trace openapi request and response, merge error and metadata error
func TraceOpenAPI[T any, R any](apiName string, openapi func() (R, *response.ResponseMetadata, error), req T) (R, error) {
	resp, metadata, err := openapi()
	logrus.Tracef("OpenAPI %s request: %+v, resp: %+v, err: %v", apiName, req, resp, err)
	if err != nil || metadata.Error != nil {
		return resp, fmt.Errorf("OpenAPI request error: %v, response: %+v", err, metadata)
	}
	return resp, nil
}

func escapeTXTRecordValue(value string) string {
	if strings.HasPrefix(value, "\"heritage=") {
		// remove \" in txt record value for volcengine privatezone
		return fmt.Sprintf("%s", strings.Replace(value, "\"", "", -1))
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

type LoggerAdapter struct {
	*logrus.Entry
}

func NewLoggerAdapter(logger *logrus.Entry) *LoggerAdapter {
	return &LoggerAdapter{logger}
}

func (l *LoggerAdapter) Log(args ...interface{}) {
	l.Entry.Log(l.Logger.GetLevel(), args...)
}
