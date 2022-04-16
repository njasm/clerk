package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	clerk "github.com/njasm/clerk/internal"
)

func main() {
	fmt.Println("Starting Clerk")

	stop := make(chan bool, 1)
	osSignal := make(chan os.Signal, 2)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)
	go func(stopServer chan bool, osSignal <-chan os.Signal) {
		s := <-osSignal
		println(fmt.Sprintf("RECEIVED SIGNAL: %v", s))

		stopServer <- true
	}(stop, osSignal)

	server, err := clerk.New(stop)
	ExitOnError(err)

	server.Start()

}

func ExitOnError(e error) {
	if e != nil {
		err := fmt.Errorf("error: %w", e)
		fmt.Println(err)
		os.Exit(1)
	}
}
