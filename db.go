package afind

import (
	"bytes"
	"fmt"
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
	// For each value matching any key with this prefix
	ForEachSuffix(keyPrefix string, f iterFunc)
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
	fmt.Printf("get key=%s\n", key)
	if repo, ok := d.r[key]; !ok {
		return nil
	} else {
		return repo
	}
}

func (d *db) Set(key string, value interface{}) error {
	fmt.Printf("set key=%s v=%#v\n", key, value)
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

func (d *db) ForEachSuffix(prefix string, ifunc iterFunc) {
	d.RLock()
	defer d.RUnlock()

	bPrefix := []byte(prefix)
	d.rt.Each(func(node *art.ArtNode) {
		if node.IsLeaf() && node.IsMatchPrefix(bPrefix) {
			key := string(bytes.TrimRight(node.Key(), "\000"))
			if value := d.get(key); value != nil {
				ifunc(key, value)
			}
		}
	})
}

func newDb() *db {
	return &db{&sync.RWMutex{}, make(map[string]*Repo), art.NewArtTree()}
}
