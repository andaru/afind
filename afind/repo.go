package afind

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
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
	Root      string `json:"root"`       // Root path
	Meta      Meta   `json:"meta"`       // Metadata for this Repo
	State     string `json:"state"`      // Current repository indexing state

	// Metadata produced during indexing
	NumFiles  int      `json:"num_files"`  // Number of files indexed
	SizeIndex ByteSize `json:"size_index"` // Size of index
	SizeData  ByteSize `json:"size_data"`  // Size of the source data

	// Number of separate index files (shards) used for this repo
	NumShards int `json:"num_shards"`

	// The time spent producing the indices for this repo
	ElapsedIndexing time.Duration `json:"elapsed"`

	// When the repo was last updated (used for repo database aging)
	TimeUpdated time.Time `json:"time_updated"`
}

// SetMeta updates the Repo's Meta first from defaults, then replace.
func (r *Repo) SetMeta(defaults Meta, replace Meta) {
	for k, v := range defaults {
		r.Meta[k] = v
	}
	for k, v := range replace {
		r.Meta[k] = v
	}
}

// SetHost sets the Repo's Meta data host value
func (r *Repo) SetHost(host string) {
	r.Meta.SetHost(host)
}

// Host returns the name of the host the Repo was indexed on
func (r *Repo) Host() string {
	return r.Meta.Host()
}

// Shards returns the Repo's slice of shard file names
func (r *Repo) Shards() []string {
	shards := make([]string, r.NumShards)
	for i := 0; i < r.NumShards; i++ {
		shards[i] = path.Join(r.IndexPath, shardName(r.Key, i))
	}
	return shards
}

// NewRepo returns an initialized empty Repo
func NewRepo() *Repo {
	return &Repo{Meta: make(Meta)}
}

// Meta is string/string map, used in indexing and search queries.
// We use a typedef to bind methods to the map because afind places
// significance in the value of certain keys (e.g., `host`), and to
// provide equality, query metadata matching and update functions.
type Meta map[string]string

// Host returns the `host` key from the metadata.
// This key is used by the afind system to denote the host containing
// a particular Repo.
func (m Meta) Host() (name string) {
	name, _ = m["host"]
	return
}

// SetHost sets the Meta's host value
func (m Meta) SetHost(host string) {
	m["host"] = host
}

// Matches checks whether some other metadata matches this.
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

// MatchesRegexp matches other metadata by regular expressions.
// Scanning of the metadata is a per the Matches function, but
// the value string of other is considered to a regular expression
// to match. If the first rune of the value string is '!', the
// remaining expression must not match. This means that each key
// of other is either a match or not match filter.
func (m Meta) MatchesRegexp(other Meta) bool {
	var err error
	for k, v := range m {
		var reg *regexp.Regexp
		var nomatch bool

		// Build the match expression for this key
		ov := other[k]
		if strings.HasPrefix(ov, "!") {
			nomatch = true
			reg, err = regexp.Compile(ov[1:])
		} else if ov != "" {
			reg, err = regexp.Compile(ov)
		}
		if err != nil || reg == nil {
			// no valid regexp, skip this key
			continue
		}
		if nomatch && reg.MatchString(v) {
			return false
		} else if !nomatch && !reg.MatchString(v) {
			return false
		}
	}
	return true
}

// Update updates this metadata from some other metadata
func (m Meta) Update(other Meta) {
	for k, v := range other {
		m[k] = v
	}
}

// newRepoFromQuery is a convenience function to create a new Repo
// from the query and the path to write index shards to.
func newRepoFromQuery(q *IndexQuery, ixpath string) *Repo {
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

// UnmarshalJSON unmarshals the byte slice of JSON into the Repo
func (r *Repo) UnmarshalJSON(b []byte) (err error) {
	newr := repo{Meta: make(Meta)}
	err = json.Unmarshal(b, &newr)
	if err == nil {
		*r = Repo(newr)
	}
	return
}

// ReposMatchingMeta returns a slice of pointers to repos matching the metadata.
func ReposMatchingMeta(repos KeyValueStorer, meta Meta, metaRegexp bool, max int) []*Repo {
	// Filter all available Repo against request
	// metadata. Values in the Meta are considered regular
	// expressions, if request.MetaRegexpMatch is set. If not
	// set, Meta values are treated as exact strings to filter
	// for. Only matching keys are considered, so filters that do
	// not appear in the Repo pass the filter.
	result := []*Repo{}
	repos.ForEach(func(key string, value interface{}) bool {
		r := value.(*Repo)
		// Skip unavailable repos
		if r.State != OK {
			return true
		}
		if !metaRegexp && r.Meta.Matches(meta) {
			// Exact string match
			result = append(result, r)
		} else if metaRegexp && r.Meta.MatchesRegexp(meta) {
			// Regular expression match
			result = append(result, r)
		}
		if max == 0 || len(result) < max {
			return true
		}
		return false
	})
	return result
}
