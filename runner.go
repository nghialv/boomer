package boomer

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/nghialv/boomer/client"
)

const (
	stateInit     = "ready"
	stateHatching = "hatching"
	stateRunning  = "running"
	stateStopped  = "stopped"

	slaveReportInterval = 3 * time.Second
)

type runner struct {
	userFactory UserFactory
	numClients  int32
	hatchRate   int
	stopChannel chan bool
	state       string
	client      client.Client
	nodeID      string

	hatchCtx    context.Context
	hatchCancel func()
}

func (r *runner) getReady() {
	r.state = stateInit
	go func() {
		for {
			msg := <-client.FromMasterCh
			switch msg.Type {
			case "hatch":
				client.ToMasterCh <- client.NewMessage("hatching", nil, r.nodeID)
				clients, hatchRate := parseHatchMessage(msg)
				if clients == 0 || hatchRate == 0 {
					log.Printf("Invalid hatch message from master, num_clients is %d, hatch_rate is %d\n", clients, hatchRate)
					continue
				}
				if r.hatchCancel != nil {
					r.hatchCancel()
				}
				r.hatchCtx, r.hatchCancel = context.WithCancel(context.Background())
				r.startHatching(r.hatchCtx, clients, hatchRate)
			case "stop":
				r.stop()
				client.ToMasterCh <- client.NewMessage("client_stopped", nil, r.nodeID)
				client.ToMasterCh <- client.NewMessage("client_ready", nil, r.nodeID)
			case "quit":
				log.Println("Got quit message from master, shutting down...")
				os.Exit(0)
			}
		}
	}()
	client.ToMasterCh <- client.NewMessage("client_ready", nil, r.nodeID)
	go func() {
		for {
			select {
			case data := <-messageToRunner:
				data["user_count"] = r.numClients
				client.ToMasterCh <- client.NewMessage("stats", data, r.nodeID)
			}
		}
	}()
	if maxRPSEnabled {
		go func() {
			for {
				atomic.StoreInt64(&maxRPSThreshold, maxRPS)
				time.Sleep(1 * time.Second)
				// Use channel to broadcast
				close(maxRPSControlCh)
				maxRPSControlCh = make(chan bool)
			}
		}()
	}
}

func parseHatchMessage(msg *client.Message) (clients int, hatchRate int) {
	rate, _ := msg.Data["hatch_rate"]
	hatchRate = int(rate.(float64))
	numClients, _ := msg.Data["num_clients"]
	if _, ok := numClients.(uint64); ok {
		clients = int(numClients.(uint64))
	} else {
		clients = int(numClients.(int64))
	}
	return
}

func (r *runner) onQuiting() {
	client.ToMasterCh <- client.NewMessage("quit", nil, r.nodeID)
	log.Println("Quit")
}

func (r *runner) startHatching(ctx context.Context, spawnCount int, hatchRate int) {
	if r.state != stateRunning && r.state != stateHatching {
		clearStatsCh <- true
		r.stopChannel = make(chan bool)
	}
	if r.state == stateRunning || r.state == stateHatching {
		// Stop previous goroutines without blocking
		// those goroutines will exit when r.safeRun returns
		close(r.stopChannel)
	}
	r.stopChannel = make(chan bool)
	r.state = stateHatching
	r.hatchRate = hatchRate
	r.numClients = 0
	go r.spawnGoRoutines(ctx, spawnCount, r.stopChannel)
}

func (r *runner) spawnGoRoutines(ctx context.Context, spawnCount int, quit chan bool) {
	log.Println("Hatching and swarming", spawnCount, "clients at the rate", r.hatchRate, "clients/s...")
	index := 0
	for index < spawnCount {
		for i := 0; i < r.hatchRate && index < spawnCount; i++ {
			go func(id int) {
				atomic.AddInt32(&r.numClients, 1)
				defer func() {
					atomic.AddInt32(&r.numClients, -1)
				}()
				user := r.userFactory(id)
				tr := &taskRunner{
					user: user,
				}
				tr.Run(ctx)
			}(index)
			index++
		}
		time.Sleep(time.Second)
	}
	log.Printf("Hatching completed, %d users are created", index)
	r.hatchComplete()
}

func (r *runner) hatchComplete() {
	data := map[string]interface{}{
		"count": r.numClients,
	}
	client.ToMasterCh <- client.NewMessage("hatch_complete", data, r.nodeID)
	r.state = stateRunning
}

func (r *runner) stop() {
	if r.hatchCancel != nil {
		r.hatchCancel()
	}
	if r.state == stateRunning || r.state == stateHatching {
		close(r.stopChannel)
		r.state = stateStopped
		log.Println("Recv stop message from master, all the goroutines are stopped")
	}
}
