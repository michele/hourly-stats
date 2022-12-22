package hourly_stats

import (
	"bytes"
	"encoding/gob"
	"strings"
	"sync"
	"time"
)

const hourlyBucketFormat = "06010215"

type Stats struct {
	Data map[string]*bucket
	lock *sync.Mutex
}

type bucket struct {
	Data map[string]*hourly
	lock *sync.Mutex
}

type hourly struct {
	Data map[string]int
	lock *sync.Mutex
}

func NewStats() *Stats {
	return &Stats{
		Data: map[string]*bucket{},
		lock: &sync.Mutex{},
	}
}

func newBucket() *bucket {
	return &bucket{
		Data: map[string]*hourly{},
		lock: &sync.Mutex{},
	}
}

func newHourly() *hourly {
	return &hourly{
		Data: map[string]int{},
		lock: &sync.Mutex{},
	}
}

func (h *hourly) Incr(hour string) {
	if h.lock == nil {
		h.lock = &sync.Mutex{}
	}
	h.lock.Lock()
	t, ok := h.Data[hour]
	if !ok {
		t = 0
	}
	h.Data[hour] = t + 1
	h.lock.Unlock()
}

func (b *bucket) Incr(ref string, hour string) {
	if b.lock == nil {
		b.lock = &sync.Mutex{}
	}
	b.lock.Lock()
	h, ok := b.Data[ref]
	if !ok {
		h = newHourly()
	}
	h.Incr(hour)
	b.Data[ref] = h
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
	if s.lock == nil {
		s.lock = &sync.Mutex{}
	}
	s.lock.Lock()
	b, ok := s.Data[buck]
	if !ok {
		b = newBucket()
	}
	b.Incr(sub, hour)
	s.Data[buck] = b
	s.lock.Unlock()
}

type Report struct {
	Total         int                `json:"total"`
	Average       float64            `json:"average"`
	HourlyAverage map[int]float64    `json:"hourly_average,omitempty"`
	LastDay       []int              `json:"last_day,omitempty"`
	Subs          map[string]*Report `json:"subs,omitempty"`
	keys          map[string]bool
}

func (s *Stats) Stats() *Report {
	subs := map[string]*Report{}
	for k, v := range s.Data {
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
	lastDay := make([]int, 24)
	for k, v := range b.Data {
		r := v.stats()
		total += r.Total
		for h, _ := range r.keys {
			hkeys[h] = true
		}
		subs[k] = r
		for i, d := range r.LastDay {
			lastDay[i] += d
		}
	}
	return &Report{
		Total:   total,
		Average: float64(total) / float64(len(hkeys)),
		Subs:    subs,
		LastDay: lastDay,
	}
}

func (h *hourly) stats() *Report {
	now24 := time.Now().Add(-24 * time.Hour)
	currentH := time.Now().Hour()
	total := 0
	hours := map[int]int{}
	hoursN := map[int]int{}
	keys := map[string]bool{}
	lastDay := make([]int, 24)
	for k, v := range h.Data {
		total += v
		t, err := time.Parse(hourlyBucketFormat, k)
		if err != nil {
			continue
		}
		if t.After(now24) {
			lastInd := ((t.Hour() - currentH) - 24) % 24
			if lastInd < 0 {
				lastInd *= -1
			}
			lastDay[lastInd] = v
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
		Average:       float64(total) / float64(len(h.Data)),
		HourlyAverage: havg,
		LastDay:       lastDay,
		keys:          keys,
	}
}

func (s *Stats) Dump() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(s)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewStatsFromDump(bts []byte) (*Stats, error) {
	buf := bytes.NewBuffer(bts)
	dec := gob.NewDecoder(buf)
	var s Stats
	err := dec.Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
