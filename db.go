package afind

import (
	"sync"

	"github.com/andaru/go-art"
)

type db struct {
	*sync.RWMutex
	r  map[string]*Repo
	rt *art.ArtTree
}

type iterFunc func(key string, value interface{}) bool

type KeyValueStorer interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	Delete(key string) error

	Size() int

	// Iteration primitives
	ForEach(f iterFunc)
}

func (d *db) Size() int {
	d.RLock()
	defer d.RUnlock()
	return len(d.r)
}

func (d *db) Get(key string) interface{} {
	d.RLock()
	defer d.RUnlock()
	return d.get(key)
}

func (d *db) get(key string) interface{} {
	if repo, ok := d.r[key]; !ok {
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

	d.r[key] = value.(*Repo)
	d.rt.Insert([]byte(key), value)
	return nil
}

func (d *db) Delete(key string) error {
	d.Lock()
	defer d.Unlock()
	delete(d.r, key)
	d.rt.Remove([]byte(key))
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
	return &db{&sync.RWMutex{}, make(map[string]*Repo), art.NewArtTree()}
}
