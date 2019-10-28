package daemon

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type DemoWorker struct {
	StoppableByChan

	Msg string
}

func NewDemoWorker(msg string) *DemoWorker {
	sc := make(chan *sync.WaitGroup, 1)
	return &DemoWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		Msg: msg,
	}
}

func (s *DemoWorker) DemoWorker() {
	log.Info("Starting Demo Worker...")
	for {
		select {
		case <-time.After(5 * time.Second):
			log.Info(s.Msg)

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Demo Worker stopping")

			<-time.After(3 * time.Second)
			wg.Done()
			return
		}
	}
}
