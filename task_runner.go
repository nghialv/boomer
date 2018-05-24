package boomer

import (
	"context"
	"fmt"
	"math/rand"
	"runtime/debug"
	"time"
)

type taskRunner struct {
	user User
}

func (r *taskRunner) Run(ctx context.Context) {
	tasks := r.user.Tasks()
	if len(tasks) == 0 {
		return
	}
	waitTime := func() time.Duration {
		config := r.user.Config()
		dw := config.MaxWait - config.MinWait
		if dw == 0 {
			return config.MinWait
		}
		r := rand.Int63n(int64(dw))
		return time.Duration(r) + config.MinWait
	}
	index := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			startTime := time.Now()
			task := tasks[index]
			recoverableRun(func() {
				task.Fn(ctx)
			})
			wait := waitTime() - time.Since(startTime)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				timer.Stop()
			}
			index = (index + 1) % len(tasks)
		}
	}
}

func recoverableRun(fn func()) {
	defer func() {
		if err := recover(); err != nil {
			debug.PrintStack()
			events.Publish("request_failure", "unknown", "panic", 0.0, fmt.Sprintf("%v", err))
		}
	}()
	fn()
}

// weightSum := 0
// for _, task := range r.tasks {
// 	weightSum += task.Weight
// }
// for _, task := range r.tasks {
// 	percent := float64(task.Weight) / float64(weightSum)
// 	amount := int(round(float64(spawnCount)*percent, .5, 0))
// 	if weightSum == 0 {
// 		amount = int(float64(spawnCount) / float64(len(r.tasks)))
// 	}
// 	for i := 1; i <= amount; i++ {
// 		select {
// 		case <-quit:
// 			// Quit hatching goroutine
// 			return
// 		default:
// 			if i%r.hatchRate == 0 {
// 				time.Sleep(1 * time.Second)
// 			}
// 			atomic.AddInt32(&r.numClients, 1)
// 			go func(fn func()) {
// 				for {
// 					select {
// 					case <-quit:
// 						return
// 					default:
// 						if maxRPSEnabled {
// 							token := atomic.AddInt64(&maxRPSThreshold, -1)
// 							if token < 0 {
// 								// Max RPS is reached, wait until next second
// 								<-maxRPSControlCh
// 							} else {
// 								r.safeRun(fn)
// 							}
// 						} else {
// 							r.safeRun(fn)
// 						}
// 					}
// 				}
// 			}(task.Fn)
// 		}
// 	}
// }
