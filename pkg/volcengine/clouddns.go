package volcengine

import "github.com/volcengine/volc-sdk-golang/service/dns"

type cloudZoneAPI interface {
}

type czWrapper struct {
	client *dns.Client
}

func NewCzWrapper() *czWrapper {
	return &czWrapper{
		client: dns.NewClient(dns.NewVolcCaller()),
	}
}
