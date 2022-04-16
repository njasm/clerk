package registry

import (
	clerk "github.com/njasm/clerk/internal"
)

const consulID = "consul"

type Consul struct{}

func (c *Consul) ID() string {
	return consulID
}

func (c *Consul) Ping() error {
	return nil
}

func (c *Consul) Register(service *clerk.Service) error {
	if service == nil {
		return nil
	}

	return nil
}

func (c *Consul) Unregister(service *clerk.Service) error {
	return nil
}

func (c *Consul) Refresh(service *clerk.Service) error {
	return nil
}
