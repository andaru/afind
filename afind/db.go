package afind

import (
	"encoding/json"
	"io"
	"os"
	"sync"
)

// A KeyValueStorer is a simple key/value store, used in afind to
// store Repo in memory (backed to files).
type KeyValueStorer interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	Delete(key string) error
	Size() int
	ForEach(f IterFunc)
}

// An optionally file backed key/value store implementing
// KeyValueStorer
type db struct {
	*sync.RWMutex

	R map[string]*Repo `json:"repo"` // map of repo key to repo

	bfn string             // backing filename
	bs  io.ReadWriteCloser // backing store
}

const (
	writeOptions = os.O_CREATE | os.O_TRUNC | os.O_RDWR
	writeMode    = 0644
	writeDirMode = 0755
)

// caller must hold the mutex when calling
func (d *db) flush() error {
	if d.bfn == "" {
		return nil
	}
	file, err := os.OpenFile(d.bfn, writeOptions, writeMode)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	return enc.Encode(d)
}

// caller must hold the mutex, and read is only called once at the
// beginning so keep that hidden assumption in mind
func (d *db) read() error {
	if d.bfn == "" {
		return nil // todo: really it's an error...
	}
	file, err := os.Open(d.bfn)
	if err == nil {
		d.bs = file
	} else {
		if !os.IsNotExist(err) {
			log.Debug("read error: %v", err)
			return nil
		}
		return err
	}
	dec := json.NewDecoder(d.bs)
	err = dec.Decode(d)

	// todo: xxx
	// convert up the repos size values
	// if err == nil {
	// 	for k, r := range d.R {
	// 		d.R[k].SizeIndex = r.SizeIndex
	// 		d.R[k].SizeData = r.SizeData
	// 	}
	// }

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
	flush := func() {
		_ = d.flush()
	}
	defer flush()

	d.R[key] = value.(*Repo)
	return nil
}

func (d *db) Delete(key string) error {
	d.Lock()
	defer d.Unlock()
	flush := func() {
		_ = d.flush()
	}
	defer flush()

	delete(d.R, key)
	return nil
}

// An iteration function wrapper. Retrun false to
// stop iteration early. Used by KeyValueStorer.ForEach.
type IterFunc func(key string, value interface{}) bool

func (d *db) ForEach(f IterFunc) {
	for key, _ := range d.R {
		if v := d.Get(key); v != nil {
			if !f(key, v) {
				return
			}
		}
	}
}

func newDb() *db {
	return &db{&sync.RWMutex{}, make(map[string]*Repo), "", nil}
}

func NewDb() *db {
	return newDb()
}

func (d *db) getSizes() (index, data ByteSize) {
	for _, v := range d.R {
		index += ByteSize(v.SizeIndex)
		data += ByteSize(v.SizeData)
	}
	return
}

func NewJsonBackedDb(filename string) *db {
	return newJsonBackedDb(filename)
}

func newJsonBackedDb(filename string) *db {
	var err error
	newDb := &db{&sync.RWMutex{}, make(map[string]*Repo),
		filename, nil}
	if err = newDb.read(); err == nil {
		sizeIndex, sizeData := newDb.getSizes()
		log.Info("Loaded database (%d repos; %s data/%s index)",
			len(newDb.R), sizeData, sizeIndex)
	} else {
		log.Info("Starting with fresh backing store (due to: %v)", err)
	}
	return newDb
}