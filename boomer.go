package boomer

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nghialv/boomer/client"
)

var (
	maxRPS          int64
	maxRPSThreshold int64
	maxRPSEnabled   = false
	maxRPSControlCh = make(chan bool)
)

type UserFactory func(id int) User

type User interface {
	Config() *UserConfig
	Tasks() []*Task
}

type UserConfig struct {
	MinWait time.Duration
	MaxWait time.Duration
}

type Task struct {
	Name   string
	Weight int
	Fn     func(context.Context)
}

type boomer struct {
	maxRPS        int
	masterAddress string
}

func init() {
	flag.Int64Var(&maxRPS, "max-rps", 0, "Max RPS that boomer can generate.")
}

func NewBoomer() *boomer {
	return &boomer{}
}

func (b *boomer) Run(factory UserFactory) {
	if !flag.Parsed() {
		flag.Parse()
	}
	if maxRPS > 0 {
		log.Println("Max RPS that boomer may generate is limited to", maxRPS)
		maxRPSEnabled = true
	}
	runner := &runner{
		userFactory: factory,
		client:      client.NewClient(),
		nodeID:      getNodeID(),
	}
	events.Subscribe("boomer:quit", runner.onQuiting)
	runner.getReady()
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT)
	<-ch
	events.Publish("boomer:quit")
	<-client.DisconnectedFromMasterCh
	log.Println("Shutdown")
}

func (b *boomer) Report(topic string, args ...interface{}) {
	events.Publish(topic, args...)
}
