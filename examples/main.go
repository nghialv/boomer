package main

import (
	"context"
	"log"
	"time"

	"github.com/nghialv/boomer"
)

var b = boomer.NewBoomer()

type User struct {
	id int
}

func newUser(id int) boomer.User {
	user := &User{
		id: id,
	}
	log.Printf("User %d created", id)
	return user
}

func (u *User) Config() *boomer.UserConfig {
	return &boomer.UserConfig{
		MinWait: 5 * time.Second,
		MaxWait: 5 * time.Second,
	}
}

func (u *User) Tasks() []*boomer.Task {
	return []*boomer.Task{
		&boomer.Task{
			Name:   "foo",
			Weight: 10,
			Fn:     u.foo,
		},
		&boomer.Task{
			Name:   "bar",
			Weight: 20,
			Fn:     u.bar,
		},
	}
}

func (u *User) foo(ctx context.Context) {
	log.Printf("%v: User %d: Foo begin", time.Now(), u.id)
	start := boomer.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := boomer.Now() - start
	// Report your test result as a success, if you write it in python, it will looks like this
	// events.request_success.fire(request_type="http", name="foo", response_time=100, response_length=10)
	b.Report("request_success", "http", "foo", elapsed, int64(10))
	log.Printf("%v: User %d: Foo end", time.Now(), u.id)
}

func (u *User) bar(ctx context.Context) {
	log.Printf("%v: User %d: Bar begin", time.Now(), u.id)
	start := boomer.Now()
	time.Sleep(100 * time.Millisecond)
	elapsed := boomer.Now() - start
	// Report your test result as a failure, if you write it in python, it will looks like this
	// events.request_failure.fire(request_type="udp", name="bar", response_time=100, exception=Exception("udp error"))
	b.Report("request_failure", "udp", "bar", elapsed, "udp error")
	log.Printf("%v: User %d: Bar end", time.Now(), u.id)
}

func main() {
	b.Run(newUser)
}
