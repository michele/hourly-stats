package hourly_stats

import (
	"time"

	"github.com/rs/zerolog/log"
	bolt "go.etcd.io/bbolt"
)

type DB struct {
	stats *Stats
	db    *bolt.DB
	path  string
	quit  chan struct{}
}

var statsBucketName = []byte("fshrmn-hourly-stats")
var statsKeyName = []byte("hstats")

func NewDB(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(statsBucketName)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var stats *Stats
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(statsBucketName)
		bts := b.Get(statsKeyName)
		if bts != nil {
			stats, err = NewStatsFromDump(bts)
			if err != nil {
				return err
			}
		} else {
			stats = NewStats()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s := DB{
		path:  path,
		stats: stats,
		db:    db,
		quit:  make(chan struct{}),
	}
	return &s, nil
}

func (db *DB) Close() error {
	close(db.quit)
	err := db.Dump()
	if err != nil {
		return err
	}
	return db.db.Close()
}

func (db *DB) Dump() error {
	bts, err := db.stats.Dump()
	if err != nil {
		return err
	}
	err = db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(statsBucketName)
		return b.Put(statsKeyName, bts)
	})
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) Start() {
	go func() {
		t := time.NewTicker(10 * time.Minute)
		for {
			select {
			case <-db.quit:
				t.Stop()
				return
			case <-t.C:
				err := db.Dump()
				if err != nil {
					log.Error().Err(err).Msgf("Couldn't dump: %+v", err)
				}
			}
		}
	}()
}

func (db *DB) Incr(ref string) {
	db.stats.Incr(ref)
}

func (db *DB) Report(ref string) *Report {
	buck, ok := db.stats.Data[ref]
	if ok {
		return buck.stats()
	}
	return &Report{}
}
