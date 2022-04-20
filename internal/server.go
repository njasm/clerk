package clerk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerapi "github.com/docker/docker/client"
	"github.com/njasm/clerk/internal/service"
	"github.com/njasm/clerk/internal/utils"
)

type ContainerID string
type ServiceID string

type OperationType int

const (
	OP_REGISTER OperationType = iota + 1
	OP_UNREGISTER
	OP_LIST_ALL
)

type TrackMessage struct {
	operation OperationType
	id        string
	reply     chan []string
}

func newRegisterServiceMessage(id string) *TrackMessage {
	return &TrackMessage{
		operation: OP_REGISTER,
		id:        id,
	}
}

func newUnregisterServiceMessage(id string) *TrackMessage {
	return &TrackMessage{
		operation: OP_UNREGISTER,
		id:        id,
	}
}

func newListAllServiceMessage() *TrackMessage {
	return &TrackMessage{
		operation: OP_LIST_ALL,
		reply:     make(chan []string, 1),
	}
}

// trackRegisteredServices track in memory all services registered by this instance of clerk
func trackRegisteredServices(chMessage <-chan *TrackMessage, stopServer <-chan bool) {
	store := map[string]struct{}{}
	for {
		select {
		case message, ok := <-chMessage:
			if !ok {
				fmt.Println("-- CLOSE: chMessage --")
				continue
			}

			if message.operation == OP_REGISTER {
				fmt.Printf("-- REGISTER: %s --\n", message.id)
				store[message.id] = struct{}{}
				continue
			}

			if message.operation == OP_UNREGISTER {
				fmt.Printf("-- UNREGISTER: %s --\n", message.id)
				delete(store, message.id)
				continue
			}

			if message.operation == OP_LIST_ALL {
				fmt.Printf("-- LIST_ALL: %s --\n", message.id)
				data := []string{}
				for key := range store {
					data = append(data, key)
				}

				message.reply <- data
			}
		case <-stopServer:
			fmt.Println("-- SIGNAL RECEIVED, Exiting...")
			return
		}
	}
}

type Server struct {
	dockerClient          DockerAPIClient
	dockerMessagesChannel <-chan events.Message
	dockerErrorChannel    <-chan error
	stopServerChannel     <-chan bool
	registry              Registry
	trackServicesChannel  chan<- *TrackMessage
}

func New(stopServer <-chan bool, registry Registry) (*Server, error) {
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

	chTrack := make(chan *TrackMessage, 64)

	go trackRegisteredServices(chTrack, stopServer)

	return &Server{
		dockerClient:          client,
		dockerMessagesChannel: chMessages,
		dockerErrorChannel:    chErrors,
		stopServerChannel:     stopServer,
		registry:              registry,
		trackServicesChannel:  chTrack,
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

			if data.Status == "start" && data.Type == "container" {
				err := s.register(data.Actor.ID)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					fmt.Println(err)
				}
			}

			if data.Status == "die" && data.Type == "container" {
				err := s.unregister(data.Actor.ID)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					log.Println(err)
				}
			}

		case e := <-s.dockerErrorChannel:

			err := fmt.Errorf("error: %w", e)
			fmt.Println(err)

		case <-timer.C:

			fmt.Println("TIMER TICK - SYNCHRONIZATION")

			containers, err := s.dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
			if err != nil {
				err = fmt.Errorf("error: %w", err)
				fmt.Println(err)
				continue
			}

			err = s.synchronise(containers)
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

func (s *Server) register(containerID string) error {
	service, err := s.containerToService(containerID)
	if err != nil {
		return err
	}

	if !service.Register() {
		return nil
	}

	err = s.registry.Register(service)
	if err != nil {
		log.Println(fmt.Errorf("error registering service: %w", err))
		err = nil
	}

	s.trackServicesChannel <- newRegisterServiceMessage(service.ID())
	return nil
}

var ErrIsClosed = errors.New("chan is closed")

func (s *Server) unregister(containerID string) error {
	service, err := s.containerToService(containerID)
	if err != nil {
		return err
	}

	err = s.registry.Unregister(service)
	if err != nil {
		log.Println(fmt.Errorf("error unregistering service: %w", err))
		err = nil
	}

	s.trackServicesChannel <- newUnregisterServiceMessage(service.ID())
	return nil
}

func (s *Server) containerToService(containerID string) (*service.Service, error) {
	containerJson, err := s.dockerClient.ContainerInspect(context.TODO(), containerID)
	if err != nil {
		return nil, err
	}

	return service.NewFrom(containerJson), nil
}

func (s *Server) synchronise(containers []types.Container) error {
	if len(containers) == 0 {
		return nil
	}

	trackMessage := newListAllServiceMessage()
	s.trackServicesChannel <- trackMessage
	regServices, ok := <-trackMessage.reply
	if !ok {
		// channel is closed ?!?
		err := fmt.Errorf("error: %w", ErrIsClosed)
		fmt.Println(err)
		return err
	}

	// TODO: we need to simplify this piece of code, we're doing too much here
	tracked := map[ServiceID]ContainerID{}
	var group sync.WaitGroup
	for _, container := range containers {
		containerID := ContainerID(container.ID)
		srv, err := s.containerToService(container.ID)
		if err != nil {
			continue
		}

		// we have tracked this container?
		for _, instance := range srv.Instances() {
			if utils.Any(regServices, instance.ID) {
				serviceID := ServiceID(instance.ID)
				if _, exists := tracked[serviceID]; !exists {
					tracked[serviceID] = containerID
				}

				continue
			}

			group.Add(1)
			go func(s *Server, id string, wg *sync.WaitGroup) {
				err := s.register(id)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					fmt.Println(err)
				}

				wg.Done()
			}(s, container.ID, &group)

			break
		}
	}

	services, err := s.registry.Services()
	if err != nil {
		err = fmt.Errorf("error getting services from registry: %w", err)
		fmt.Println(err)
		return err
	}

	registeredContainer := map[ContainerID]bool{}
	mapperFn := func(v *service.RegisteredService) string { return v.ID }
	// and its still registered in the registry?
	for _, cID := range tracked {
		if _, exists := registeredContainer[cID]; exists {
			continue
		}

		for _, id := range utils.Map(services, mapperFn) {
			if _, exists := tracked[ServiceID(id)]; exists {
				continue
			}

			registeredContainer[cID] = true
			group.Add(1)
			go func(s *Server, id string, wg *sync.WaitGroup) {
				err := s.register(id)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					fmt.Println(err)
				}

				wg.Done()
			}(s, string(cID), &group)

			break
		}
	}

	group.Wait()

	return nil
}

func ExitOnError(e error) {
	if e != nil {
		err := fmt.Errorf("error: %w", e)
		fmt.Println(err)
		os.Exit(1)
	}
}
