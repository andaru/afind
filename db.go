package afind

// db provides the KeyValueStore and Iterator interfaces
// and implementations

import (
	"sync"
)

type KeyValueStore interface {
	Get(string) (interface{}, bool)
	Set(string, interface{}) error
	Delete(string) error
	Size() int
}

type IterableKeyValueStore interface {
	KeyValueStore
	Iterator
}

type IterFunc func(key string, value interface{}) bool

type Iterator interface {
	ForEach(IterFunc) error
}

type kvstore struct {
	sync.RWMutex
	d map[string]interface{}
}

func NewKvstore() kvstore {
	return kvstore{d:make(map[string]interface{})}
}

func (s *kvstore) Size() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.d)
}

func (s *kvstore) Get(k string) (v interface{}, ok bool) {
	s.RLock()
	defer s.RUnlock()

	v, ok = s.d[k]
	return

}

func (s *kvstore) Set(k string, v interface{}) error {
	s.Lock()
	defer s.Unlock()

	s.d[k] = v
	return nil
}

func (s *kvstore) Delete(k string) error {
	s.Lock()
	defer s.Unlock()

	delete(s.d, k)
	return nil
}

func (s *kvstore) ForEach(iter IterFunc) error {
	for k, v := range s.d {
		if !iter(k, v) {
			break
		}
	}
	return nil
}
