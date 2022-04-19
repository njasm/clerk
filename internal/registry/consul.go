package registry

import (
	"errors"
	"fmt"
	"log"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	clerk "github.com/njasm/clerk/internal"
	service "github.com/njasm/clerk/internal/service"
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

func (c *Consul) Register(service *service.Service) error {
	if service == nil {
		return ErrServiceIsNil
	}

	check := agentServiceCheck(service)
	config := convertMetadataKeys(service.Config())
	for _, instance := range service.Instances() {
		registration := consulapi.AgentServiceRegistration{
			Kind:    consulapi.ServiceKindTypical,
			ID:      instance.ID,
			Address: instance.IP,
			Port:    instance.Port,
			Name:    service.Name(),
			Tags:    service.Tags(),
			Meta:    config,
			Check:   check,
		}

		err := c.client.Agent().ServiceRegister(&registration)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Consul) Unregister(service *service.Service) error {
	if service == nil {
		return ErrServiceIsNil
	}

	return c.client.Agent().ServiceDeregister(service.ID())
}

func (c *Consul) Refresh(service *service.Service) error {
	return nil
}

func (c *Consul) Services() ([]*service.RegisteredService, error) {
	rv := []*service.RegisteredService{}
	services, err := c.client.Agent().Services()
	if err != nil {
		return rv, err
	}

	for _, value := range services {
		config := revertMetadataKeys(value.Meta)
		s := &service.RegisteredService{
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

func agentServiceCheck(service *service.Service) *consulapi.AgentServiceCheck {
	check := new(consulapi.AgentServiceCheck)
	if path, ok := service.GetConfig("consul.check.http"); ok {
		check.HTTP = fmt.Sprintf("http://%s:%d%s", service.IPAddress(), service.Port(), path)
	}

	if path, ok := service.GetConfig("consul.check.https"); ok {
		check.HTTP = fmt.Sprintf("https://%s:%d%s", service.IPAddress(), service.Port(), path)
	}

	if check.HTTP != "" {
		if method, ok := service.GetConfig("consul.check.method"); ok {
			check.Method = method
		}
	}

	if _, ok := service.GetConfig("consul.check.tcp"); ok {
		check.TCP = fmt.Sprintf("%s:%d", service.IPAddress(), service.Port())
	}

	if _, ok := service.GetConfig("consul.check.grpc"); ok {
		check.GRPC = fmt.Sprintf("%s:%d", service.IPAddress(), service.Port())
		if useTLS, ok := service.GetConfig("consule.check.grpc.tls"); ok {
			if strings.Trim(strings.ToLower(useTLS), " ") == "true" {
				check.GRPCUseTLS = true
				if tlsSkipVerify, ok := service.GetConfig("consul.check.tls.skip.verify"); ok {
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
		if timeout, ok := service.GetConfig("consul.check.timout"); ok {
			check.Timeout = timeout
		} else {
			check.Timeout = "2s"
		}

		if interval, ok := service.GetConfig("consul.check.interval"); ok {
			check.Interval = interval
		} else {
			check.Interval = "10s"
		}
	}

	if after, ok := service.GetConfig("consul.check.deregister.after"); ok {
		check.DeregisterCriticalServiceAfter = after
	}

	return check
}
