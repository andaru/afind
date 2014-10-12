package afind

import (
	"encoding/json"
	"io"
	"os"
	"sync"
)

type iterFunc func(key string, value interface{}) bool

type KeyValueStorer interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	Delete(key string) error

	Size() int

	// Iteration primitives
	ForEach(f iterFunc)
}

// An optionally file backed key/value store
type db struct {
	*sync.RWMutex

	R map[string]*Repo // repo key to repo map

	bfn string             // backing filename
	bs  io.ReadWriteCloser // backing store
}

// caller must hold the mutex
func (d *db) flush() error {
	if d.bfn == "" {
		return nil
	}
	enc := json.NewEncoder(d.bs)
	return enc.Encode(d)
}

func (d *db) openbs() error {
	if file, err := os.OpenFile(d.bfn, os.O_CREATE|os.O_RDWR, 0640); err != nil {
		return err
	} else {
		d.bs = file
		return nil
	}
}

// caller must hold the mutex
func (d *db) read() error {
	if d.bfn == "" {
		return nil // todo: really it's an error...
	}
	var err error
	dec := json.NewDecoder(d.bs)
	err = dec.Decode(d)
	return err
}

func (d *db) close() error {
	if err := d.flush(); err != nil {
		log.Critical("failed to flush backing store: %v", err.Error())
		return err
	}
	if err := d.bs.Close(); err != nil {
		log.Critical("failed to close backing store: %v", err.Error())
		return err
	}
	return nil
}

func (d *db) Size() int {
	d.RLock()
	defer d.RUnlock()
	return len(d.R)
}

func (d *db) Get(key string) interface{} {
	d.RLock()
	defer d.RUnlock()
	return d.get(key)
}

func (d *db) get(key string) interface{} {
	if repo, ok := d.R[key]; !ok {
		return nil
	} else {
		return repo
	}
}

func (d *db) Set(key string, value interface{}) error {
	if value == nil {
		return d.Delete(key)
	}
	d.Lock()
	defer d.Unlock()
	defer d.flush()

	d.R[key] = value.(*Repo)
	return nil
}

func (d *db) Delete(key string) error {
	d.Lock()
	defer d.Unlock()
	defer d.flush()
	delete(d.R, key)
	return nil
}

func (d *db) ForEach(f iterFunc) {
	for key, _ := range d.R {
		if v := d.Get(key); v != nil {
			if !f(key, v) {
				return
			}
		}
	}
}

func NewEmptyDb() *db {
	return newDb()
}

func newDb() *db {
	return &db{&sync.RWMutex{}, make(map[string]*Repo), "", nil}
}

func newDbWithJsonBacking(filename string) *db {
	var err error

	newDb := &db{&sync.RWMutex{}, make(map[string]*Repo), filename, nil}
	if err = newDb.openbs(); err != nil {
		log.Fatal(err.Error())
	}
	if err = newDb.read(); err == nil {
		log.Info("Loaded database %s (%d repos)", filename, len(newDb.R))
	} else {
		log.Critical(err.Error())
	}
	return newDb
}
