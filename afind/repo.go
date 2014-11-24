package afind

import (
	"encoding/json"
	"path"
	"strconv"
	"time"
)

// Repo states. Only OK repositories will be searched.
const (
	NULL     = "NULL"
	INDEXING = "INDEXING" // currently indexing
	OK       = "OK"       // available for searching
	ERROR    = "ERROR"    // not available for searching
)

// A Repo represents a single indexed repository of source code.
//
// A repository's posting query index consists of one or more files,
// stored in IndexPath.
type Repo struct {
	Key       string `json:"key"`        // Unique key
	IndexPath string `json:"index_path"` // Path to the .afindex index files
	Root      string `json:"root"`       // Root path (under which Dirs are rooted)
	Meta      Meta   `json:"meta"`       // User configurable metadata for this Repo
	State     string `json:"state"`      // Current repository indexing state

	// Metadata produced during indexing
	NumDirs   int      `json:"num_dirs"`   // Number of directories indexed
	NumFiles  int      `json:"num_files"`  // Number of files indexed
	SizeIndex ByteSize `json:"size_index"` // Size of index
	SizeData  ByteSize `json:"size_data"`  // Size of the source data

	// Number of shards written to disk, an optimisation to avoid a glob
	NumShards int `json:"num_shards"`

	// The time spent producing the indices for this repo
	ElapsedIndexing time.Duration `json:"elapsed"`
}

func (r *Repo) SetMeta(defaults Meta, replace Meta) {
	for k, v := range defaults {
		r.Meta[k] = v
	}
	for k, v := range replace {
		r.Meta[k] = v
	}
}

func (r *Repo) SetHost(host string) {
	r.Meta["host"] = host
}

// Host returns the name of the host the Repo was indexed on
func (r *Repo) Host() string {
	return r.Meta.Host()
}

// PortRpc returns the RPC port of the Repo's indexer
func (r *Repo) PortRpc() string {
	return r.Meta["port.rpc"]
}

// Shards returns the Repo's slice of shard file names
func (r *Repo) Shards() []string {
	shards := make([]string, r.NumShards)
	for i := 0; i < r.NumShards; i++ {
		fname := r.Key + "-" + strconv.Itoa(i) + ".afindex"
		shards[i] = path.Join(r.IndexPath, fname)
	}
	return shards
}

// NewRepo returns an empty initialized Repo
func NewRepo() *Repo {
	return &Repo{Meta: make(Meta)}
}

// Meta is string/string metadata used in queries
type Meta map[string]string

// Host returns the "host" key from the metadata.
// This key is used by the afind system to denote the host containing
// a particular Repo.
func (m Meta) Host() string {
	if v, ok := m["host"]; !ok {
		return ""
	} else {
		return v
	}
}

func (m Meta) SetHost(host string) {
	m["host"] = host
}

// Checks whether some other metadata matches this.
// Each key in the local object is scanned, and a match occurs either
// when the key does not exist in the other object, or the key does
// exist in the other object and the key's value in the local object
// matches the key's value in the other object.
func (m Meta) Matches(other Meta) bool {
	for k, v := range m {
		if ov, exists := other[k]; exists {
			if v != ov {
				return false
			}
		}
	}
	return true
}

// Updates this metadata from some other metadata
func (m Meta) Update(other Meta) {
	for k, v := range other {
		m[k] = v
	}
}

// NewRepoFromQuery is a convenience function to create a new Repo
// from the query and the path to write index shards to.
func NewRepoFromQuery(q *IndexQuery, ixpath string) *Repo {
	repo := NewRepo()
	repo.Key = q.Key
	repo.Root = q.Root
	repo.IndexPath = ixpath
	for k, v := range q.Meta {
		repo.Meta[k] = v
	}
	return repo
}

// used to avoid infinite recursion in UnmarshalJSON
type repo Repo

func (r *Repo) UnmarshalJSON(b []byte) (err error) {
	newr := repo{Meta: make(Meta)}
	err = json.Unmarshal(b, &newr)
	if err == nil {
		*r = Repo(newr)
	}
	return
}
