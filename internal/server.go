package clerk

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	consulapi "github.com/hashicorp/consul/api"
)

type Server struct {
	dockerClient          DockerAPIClient
	dockerMessagesChannel <-chan events.Message
	dockerErrorChannel    <-chan error
	stopServerChannel     <-chan bool
	consulClient          *consulapi.Client
}

func New(stopServer <-chan bool) (*Server, error) {
	client, err := dockerapi.NewClientWithOpts(dockerapi.FromEnv)
	if err != nil {
		return nil, err
	}

	chMessages, chErrors := client.Events(context.TODO(), types.EventsOptions{
		Filters: filters.NewArgs(
			filters.KeyValuePair{
				Key: "Type", Value: "container",
			},
		),
	})

	consulClient, err := GetConsulClient()
	if err != nil {
		return nil, err
	}

	return &Server{
		dockerClient:          client,
		dockerMessagesChannel: chMessages,
		dockerErrorChannel:    chErrors,
		stopServerChannel:     stopServer,
		consulClient:          consulClient,
	}, nil
}

func (s *Server) Start() {

	timer := time.NewTicker(time.Second * 2)
	for {
		select {
		case data := <-s.dockerMessagesChannel:

			fmt.Printf("Event ID: %+v\n", data.ID)
			fmt.Printf("Status  : %+v\n", data.Status)
			fmt.Printf("Scope   : %+v\n", data.Scope)
			fmt.Printf("Type    : %+v\n", data.Type)
			fmt.Printf("From    : %+v\n", data.From)
			fmt.Printf("Action  : %+v\n", data.Action)
			fmt.Printf("Actor ID: %+v\n", data.Actor.ID)
			fmt.Println("A. Attrb:")

			for k, v := range data.Actor.Attributes {
				fmt.Printf(" - %+v : %+v\n", k, v)
			}

			println()
			/*
				if data.Status == "start" && data.Type == "container" {
					Register(consulClient, data)
					if err != nil {
						err = fmt.Errorf("error: %w", err)
						fmt.Println(err)
					}
				}
			*/
			if data.Status == "die" && data.Type == "container" {
				err := Deregister(s.consulClient, data.Actor.ID)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					fmt.Println(err)
				}
			}

		case e := <-s.dockerErrorChannel:

			err := fmt.Errorf("error: %w", e)
			fmt.Println(err)

		case <-timer.C:

			fmt.Println("TIMER TICK - SYNCHRONIZATION")

			containers, err := s.dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
			ExitOnError(err)

			for _, v := range containers {
				fmt.Printf("%+v\n", v)
			}

			err = Synchronise(s.consulClient, s.dockerClient, containers)
			if err != nil {
				err = fmt.Errorf("error: %w", err)
				fmt.Println(err)
			}

		case <-s.stopServerChannel:

			//TODO: teardown gracefully everywere
			fmt.Println("Tearing Down service")
			timer.Stop()
			return

		default:

			fmt.Println("No events to process")
			time.Sleep(time.Duration(time.Second * 1))
			continue

		}
	}
}

func GetConsulClient() (*consulapi.Client, error) {
	config := consulapi.DefaultConfig()
	return consulapi.NewClient(config)
}

func Register(client *consulapi.Client, data events.Message) error {
	containerName := strings.TrimLeft(data.Actor.Attributes["name"], "/")
	registration := consulapi.AgentServiceRegistration{
		Kind: consulapi.ServiceKindTypical,
		Name: containerName,
		ID:   data.Actor.ID,
	}

	return registerInAgent(client, &registration)
}

func registerInAgent(client *consulapi.Client, registration *consulapi.AgentServiceRegistration) error {
	return client.Agent().ServiceRegister(registration)
}

func Deregister(client *consulapi.Client, serviceID string) error {
	return client.Agent().ServiceDeregister(serviceID)
}

func Synchronise(client *consulapi.Client, docker DockerAPIClient, containers []types.Container) error {
	if len(containers) == 0 {
		return nil
	}

	services, err := client.Agent().Services()
	if err != nil {
		return err
	}

	for _, container := range containers {
		service, exist := services[container.ID]
		if exist {
			fmt.Printf("%+v", service)
			continue
		}

		containerJson, err := docker.ContainerInspect(context.TODO(), container.ID)
		if err != nil {
			err = fmt.Errorf("error: %w", err)
			fmt.Println(err)
			continue
		}

		serviceName := strings.TrimLeft(containerJson.Name, "/")
		for key := range containerJson.Config.ExposedPorts {
			proto, port := nat.SplitProtoPort(string(key))
			intPort, err := strconv.Atoi(port)
			if err != nil {
				continue
			}

			serviceID := fmt.Sprintf("%v:%v:%v:%v", serviceName, proto, port, containerJson.Config.Hostname)

			for _, networkValue := range containerJson.NetworkSettings.Networks {
				ipAddress := networkValue.IPAddress
				registration := consulapi.AgentServiceRegistration{
					Kind:    consulapi.ServiceKindTypical,
					Name:    serviceName,
					ID:      serviceID,
					Address: ipAddress,
					Port:    intPort,
				}

				err = registerInAgent(client, &registration)
				if err != nil {
					return err
				}
			}

		}
	}

	return nil
}

func ExitOnError(e error) {
	if e != nil {
		err := fmt.Errorf("error: %w", e)
		fmt.Println(err)
		os.Exit(1)
	}
}
