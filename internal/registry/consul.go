package registry

import (
	"errors"
	"fmt"
	"log"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	clerk "github.com/njasm/clerk/internal"
)

const consulID = "consul"

var ErrServiceIsNil = errors.New("Service is nil")

func NewConsul() (clerk.Registry, error) {
	config := consulapi.DefaultConfig()
	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating consul registry: %w", err)
	}

	return &Consul{client: client}, nil
}

type Consul struct {
	client *consulapi.Client
}

func (c *Consul) ID() string {
	return consulID
}

func (c *Consul) Ping() error {
	status := c.client.Status()
	leader, err := status.Leader()
	if err != nil {
		return err
	}

	log.Println("consul: current leader ", leader)

	return nil
}

func (c *Consul) Register(service *clerk.Service) error {
	if service == nil {
		return ErrServiceIsNil
	}

	tags := []string{}
	for key, value := range service.Config {
		if strings.ToLower(key) == "com.github.njasm.clerk.tags" {
			tags = append(tags, strings.Split(value, ",")...)
			continue
		}
	}

	check := agentServiceCheck(service)
	config := convertMetadataKeys(service.Config)
	registration := consulapi.AgentServiceRegistration{
		Kind:    consulapi.ServiceKindTypical,
		Name:    service.Name,
		ID:      service.ID,
		Address: service.IP,
		Port:    service.Port,
		Tags:    tags,
		Meta:    config,
		Check:   check,
	}

	return c.client.Agent().ServiceRegister(&registration)
}

func (c *Consul) Unregister(service *clerk.Service) error {
	if service == nil {
		return ErrServiceIsNil
	}

	return c.client.Agent().ServiceDeregister(service.ID)
}

func (c *Consul) Refresh(service *clerk.Service) error {
	return nil
}

func (c *Consul) Services() ([]*clerk.Service, error) {
	rv := []*clerk.Service{}
	services, err := c.client.Agent().Services()
	if err != nil {
		return rv, err
	}

	for _, value := range services {
		config := revertMetadataKeys(value.Meta)
		s := &clerk.Service{
			ID:     value.ID,
			Name:   value.Service,
			Port:   value.Port,
			IP:     value.Address,
			Config: config,
		}

		rv = append(rv, s)
	}

	return rv, nil
}

func convertMetadataKeys(m map[string]string) map[string]string {
	return metadataReplace(m, ".", "_")
}

func revertMetadataKeys(m map[string]string) map[string]string {
	return metadataReplace(m, "_", ".")
}

func metadataReplace(m map[string]string, old, new string) map[string]string {
	rv := map[string]string{}
	for k, v := range m {
		rv[strings.ReplaceAll(k, old, new)] = v
	}

	return rv
}

func agentServiceCheck(service *clerk.Service) *consulapi.AgentServiceCheck {
	check := new(consulapi.AgentServiceCheck)
	if path := service.GetConfig("consul.check.http"); path != "" {
		check.HTTP = fmt.Sprintf("http://%s:%d%s", service.IP, service.Port, path)
	}

	if path := service.GetConfig("consul.check.https"); path != "" {
		check.HTTP = fmt.Sprintf("https://%s:%d%s", service.IP, service.Port, path)
	}

	if check.HTTP != "" {
		if method := service.GetConfig("consul.check.method"); method != "" {
			check.Method = method
		}
	}

	if tcp := service.GetConfig("consul.check.tcp"); tcp != "" {
		check.TCP = fmt.Sprintf("%s:%d", service.IP, service.Port)
	}

	if grpc := service.GetConfig("consul.check.grpc"); grpc != "" {
		check.GRPC = fmt.Sprintf("%s:%d", service.IP, service.Port)
		if useTLS := service.GetConfig("consule.check.grpc.tls"); useTLS != "" {
			if strings.Trim(strings.ToLower(useTLS), " ") == "true" {
				check.GRPCUseTLS = true

				if tlsSkipVerify := service.GetConfig("consul.check.tls.skip.verify"); tlsSkipVerify != "" {
					if strings.Trim(strings.ToLower(tlsSkipVerify), " ") == "true" {
						check.TLSSkipVerify = true
					} else {
						check.TLSSkipVerify = false
					}
				}

			} else {
				check.GRPCUseTLS = false
				check.TLSSkipVerify = true
			}
		}
	}

	//TODO: check initial status, check cmd, check script, check TTL

	if check.HTTP != "" || check.TCP != "" || check.GRPC != "" {
		if timeout := service.GetConfig("consul.check.timout"); timeout != "" {
			check.Timeout = timeout
		} else {
			check.Timeout = "2s"
		}

		if interval := service.GetConfig("consul.check.interval"); interval != "" {
			check.Interval = interval
		} else {
			check.Interval = "10s"
		}
	}

	if after := service.GetConfig("consul.check.deregister.after"); after != "" {
		check.DeregisterCriticalServiceAfter = after
	}

	return check
}
