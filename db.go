package afind

import (
	"sync"
)

type db struct {
	*sync.RWMutex
	r map[string]*Repo
}

type iterFunc func(key string, value interface{}) bool

type KeyValueStorer interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	Delete(key string) error

	// Iteration
	ForEach(f iterFunc)
}

func (d *db) Get(key string) interface{} {
	d.RLock()
	defer d.RUnlock()

	if repo, ok := d.r[key]; !ok {
		return nil
	} else {
		return repo
	}
}

func (d *db) Set(key string, value interface{}) error {
	d.Lock()
	defer d.Unlock()

	if value == nil {
		return d.Delete(key)
	}
	d.r[key] = value.(*Repo)
	return nil
}

func (d *db) Delete(key string) error {
	delete(d.r, key)
	return nil
}

func (d *db) ForEach(f iterFunc) {
	for key, _ := range d.r {
		if v := d.Get(key); v != nil {
			if !f(key, v) {
				return
			}
		}
	}
}

func newDb() *db {
	return &db{&sync.RWMutex{}, make(map[string]*Repo)}
}
