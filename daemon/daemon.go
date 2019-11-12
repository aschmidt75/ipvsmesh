package daemon

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aschmidt75/ipvsmesh/config"
	"github.com/aschmidt75/ipvsmesh/localinterface"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// StoppableByChan is a service part that can
// be stopped by sending to given channel
type StoppableByChan struct {
	StopChan *chan *sync.WaitGroup
}

// Service is a service that is able to stop
// the background daemon (as grpc service)
type Service struct {
	StoppableByChan
	GroupID    int
	grpcServer *grpc.Server

	registeredStoppables []*StoppableByChan
	wg                   sync.WaitGroup
}

// NewService creates a new instance of the stoppable daemon service
func NewService(groupID int) *Service {
	sc := make(chan *sync.WaitGroup, 1)
	return &Service{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		GroupID: groupID,
	}
}

// Stop stops all running services
func (s *Service) Stop(context.Context, *localinterface.Empty) (*localinterface.Empty, error) {
	var timeoutSecs int = 10
	log.WithField("timeoutSecs", timeoutSecs).Info("Stopping workers...")

	// take down in reverse order
	for l, r := 0, len(s.registeredStoppables)-1; l < r; l, r = l+1, r-1 {
		s.registeredStoppables[l], s.registeredStoppables[r] = s.registeredStoppables[r], s.registeredStoppables[l]
	}

	for _, r := range s.registeredStoppables {
		if r != nil {
			*r.StopChan <- &s.wg
		}
	}

	ctxTO, cancelTO := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancelTO()

	go func() {
		s.wg.Wait()
		cancelTO()
	}()
	select {
	case <-ctxTO.Done():
	}

	s.wg.Add(1)
	*s.StopChan <- &s.wg

	return &localinterface.Empty{}, nil
}

// Register adds a new StoppableByChan to the list of registered services
func (s *Service) Register(stoppable *StoppableByChan) {
	if s.registeredStoppables == nil {
		s.registeredStoppables = make([]*StoppableByChan, 0)
	}
	s.registeredStoppables = append(s.registeredStoppables, stoppable)
	s.wg.Add(1)
}

// Start starts the background service
func (s *Service) Start(ctx context.Context) {
	log.Info("Starting services...")

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)
	signal.Notify(term, os.Interrupt)

	// start grpc stuff, kick to bgnd
	go func() {
		log.Trace("creating listener/socket file")

		log.WithFields(log.Fields{"pid": os.Getpid(), "uid": syscall.Getuid(), "gid": syscall.Getgid()}).Trace()
		syscall.Umask(int(0007)) // new files be rwxrwx---

		//
		listener, err := net.Listen("unix", config.Config().DaemonSocketPath)
		if err != nil {
			log.WithField("err", err).Error("Unable to listen on unix socket.")
			os.Exit(3)
		}

		// change ownership to group (if group given, != -1)
		if err := os.Chown(config.Config().DaemonSocketPath, -1, s.GroupID); err != nil {
			log.WithField("err", err).Warn("unable to chgrp for socket file.")
		}

		log.WithField("listener", listener).Trace("created listener, registering grpc service")

		if config.Config().TLS {
			if config.Config().TLSKeyFile == "" {
				log.Fatal("Missing --tlskey. Please supply when using --tls.")
			}

			creds, err := credentials.NewServerTLSFromFile(config.Config().TLSCertFile, config.Config().TLSKeyFile)
			if err != nil {
				log.Fatal("unable to load TLS key or certificate from file. Check --tls* parameters.")
			}

			// Create an array of gRPC options with the credentials
			opts := []grpc.ServerOption{grpc.Creds(creds)}

			s.grpcServer = grpc.NewServer(opts...)

		} else {
			s.grpcServer = grpc.NewServer()
		}

		localinterface.RegisterDaemonServiceServer(s.grpcServer, s)

		log.WithField("grpcServer", s.grpcServer).Trace("start serving grpc")

		err = s.grpcServer.Serve(listener)
		if err != nil {
			log.WithField("err", err).Error("Error serving grpc backend.")
			os.Exit(4)
		}
	}()

	select {
	case <-term:
	case <-*s.StopChan:
	}

	//
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// remove socket file if its still there.
	_, err := os.Stat(config.Config().DaemonSocketPath)
	if err == nil {
		log.Trace("removing socket file")
		err := os.Remove(config.Config().DaemonSocketPath)
		if err != nil {
			log.WithField("err", err).Error("Cannot remove unix socket file.")
		}
	}

	log.Info("Stopped.")
}
