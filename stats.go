package hourly_stats

import (
	"strings"
	"sync"
	"time"
)

const hourlyBucketFormat = "06010215"

type Stats struct {
	data map[string]*bucket
	lock *sync.Mutex
}

type bucket struct {
	data map[string]*hourly
	lock *sync.Mutex
}

type hourly struct {
	data map[string]int
	lock *sync.Mutex
}

func NewStats() *Stats {
	return &Stats{
		data: map[string]*bucket{},
		lock: &sync.Mutex{},
	}
}

func newBucket() *bucket {
	return &bucket{
		data: map[string]*hourly{},
		lock: &sync.Mutex{},
	}
}

func newHourly() *hourly {
	return &hourly{
		data: map[string]int{},
		lock: &sync.Mutex{},
	}
}

func (h *hourly) Incr(hour string) {
	h.lock.Lock()
	t, ok := h.data[hour]
	if !ok {
		t = 0
	}
	h.data[hour] = t + 1
	h.lock.Unlock()
}

func (b *bucket) Incr(ref string, hour string) {
	b.lock.Lock()
	h, ok := b.data[ref]
	if !ok {
		h = newHourly()
	}
	h.Incr(hour)
	b.data[ref] = h
	b.lock.Unlock()
}

func (s *Stats) Incr(ref string) {
	parts := strings.SplitN(ref, ".", 2)
	var buck, sub string
	if len(parts) == 1 {
		buck = "."
		sub = parts[0]
	} else {
		buck = parts[0]
		sub = parts[1]
	}
	hour := time.Now().Format(hourlyBucketFormat)
	s.lock.Lock()
	b, ok := s.data[buck]
	if !ok {
		b = newBucket()
	}
	b.Incr(sub, hour)
	s.data[buck] = b
	s.lock.Unlock()
}

type Report struct {
	Total         int                `json:"total"`
	Average       float64            `json:"average"`
	HourlyAverage map[int]float64    `json:"hourly_average,omitempty"`
	Subs          map[string]*Report `json:"subs,omitempty"`
	keys          map[string]bool
}

func (s *Stats) Stats() *Report {
	subs := map[string]*Report{}
	for k, v := range s.data {
		subs[k] = v.stats()
	}
	return &Report{
		Subs: subs,
	}
}

func (b *bucket) stats() *Report {
	total := 0
	hkeys := map[string]bool{}
	subs := map[string]*Report{}
	for k, v := range b.data {
		r := v.stats()
		total += r.Total
		for h, _ := range r.keys {
			hkeys[h] = true
		}
		subs[k] = r
	}
	return &Report{
		Total:   total,
		Average: float64(total) / float64(len(hkeys)),
		Subs:    subs,
	}
}

func (h *hourly) stats() *Report {
	total := 0
	hours := map[int]int{}
	hoursN := map[int]int{}
	keys := map[string]bool{}
	for k, v := range h.data {
		total += v
		t, err := time.Parse(hourlyBucketFormat, k)
		if err != nil {
			continue
		}
		hours[t.Hour()] += v
		hoursN[t.Hour()] += 1
		keys[k] = true
	}
	havg := map[int]float64{}
	for k, v := range hours {
		havg[k] = float64(v) / float64(hoursN[k])
	}
	return &Report{
		Total:         total,
		Average:       float64(total) / float64(len(h.data)),
		HourlyAverage: havg,
		keys:          keys,
	}
}
