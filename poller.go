// Copyright 2024 Bobby Powers. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/containerd/cgroups/v3/cgroup2"
)

type Record struct {
	Time   time.Time
	Value  uint64
	Kernel uint64
}

type Stats struct {
	Rss []Record
}

type endReq struct {
	result chan<- *Stats
}

type Poller struct {
	in    chan<- *endReq
	stats *Stats
}

func NewPoller(cgroup *cgroup2.Manager, freq int) (*Poller, error) {
	durationStr := fmt.Sprintf("%fs", 1/float64(freq))
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("bad frequency (%d): %s", freq, err)
	}
	if duration <= 0 {
		return nil, fmt.Errorf("expected positive duration, not %s", duration)
	}

	ch := make(chan *endReq)
	p := &Poller{
		in:    ch,
		stats: &Stats{},
	}

	// kick this off once at the start
	if err := p.poll(time.Now(), cgroup); err != nil {
		return nil, fmt.Errorf("poll: %s", err)
	}

	go p.poller(cgroup, ch, duration)

	return p, nil
}

func (p *Poller) poll(t time.Time, cgroup *cgroup2.Manager) error {

	stats, err := cgroup.Stat()
	if err != nil || stats == nil {
		return fmt.Errorf("cg.Stat: %s", err)
	}
	if stats.Memory == nil {
		return fmt.Errorf("cg.Stat: returned nil Memory stats")
	}

	kernel := stats.Memory.GetKernelStack() // TODO: also add pagetables, etc
	p.stats.Rss = append(p.stats.Rss, Record{t, stats.Memory.GetUsage(), kernel})

	return nil
}

// loop that runs in its own goroutine, reading stats at the desired
// frequency until shouldEnd is received
func (p *Poller) poller(cgroup *cgroup2.Manager, shouldEnd <-chan *endReq, duration time.Duration) {

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		select {
		case waiter := <-shouldEnd:
			waiter.result <- p.stats
			return
		case t := <-ticker.C:
			if err := p.poll(t, cgroup); err != nil {
				log.Printf("mstat: %s", err)
			}
		}
	}
}

func (p *Poller) End() *Stats {
	result := make(chan *Stats)
	p.in <- &endReq{result}
	return <-result
}
