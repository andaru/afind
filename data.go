package afind

import (
	"encoding/json"
	"fmt"
	"time"
)

type ByteSize float64

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

const (
	indexPathSuffix = ".afindex"
)

func (b ByteSize) String() string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.fB", b)
}

func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// For JSON, we produce the number of bytes
func (b ByteSize) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.f", b)), nil
}

// A Repo represents a single indexed repository of source code.
type Repo struct {
	Key       string            `json:"key"`          // Unique key
	IndexPath string            `json:"index_path"`   // Path to the .afindex file covering this Repo
	Root      string            `json:"root"`         // Root path (under which Dirs are rooted)
	Meta      map[string]string `json:"meta"`         // User configurable metadata for this Repo
	State     RepoState         `json:"state,string"` // Current repository indexing state

	// Metadata produced during indexing
	NumDirs   int     `json:"num_dirs,string"`   // Number of directories indexed
	NumFiles  int     `json:"num_files,string"`  // Number of files indexed
	SizeIndex float64 `json:"size_index,string"` // Size of index
	SizeData  float64 `json:"size_data,string"`  // Size of the source data
	sizeIndex ByteSize
	sizeData  ByteSize

	// Number of shards written to disk, an optimisation to avoid a glob
	NumShards int `json:"num_shards,string"`
}

// used to avoid infinite recursion in UnmarshalJSON
type repo Repo

func (r *Repo) UnmarshalJSON(b []byte) (err error) {
	newr := repo{Meta: make(map[string]string)}

	err = json.Unmarshal(b, &newr)
	if err == nil {
		*r = Repo(newr)
	}
	return
}

type RepoState int

// RepoState constants: the current state of a Repo
const (
	NULL     = iota
	INDEXING // currently indexing
	OK       // available for searching
	ERROR    // not available for searching
)

func (rs RepoState) String() string {
	switch {
	case rs == INDEXING:
		return "INDEXING"
	case rs == OK:
		return "OK"
	case rs == ERROR:
		return "ERROR"
	}
	return "NULL"
}

func (rs RepoState) MarshalJSON() (b []byte, err error) {
	return []byte(`"` + rs.String() + `"`), nil
}

func newRepo(key, uriIndex string, meta map[string]string) *Repo {
	r := &Repo{
		Key:       key,
		IndexPath: uriIndex,
		Meta:      make(map[string]string),
	}
	for k, v := range meta {
		r.Meta[k] = v
	}
	return r
}

func newRepoFromIndexRequest(request *IndexRequest) Repo {
	repo := newRepo(request.Key, "", request.Meta)
	repo.Root = request.Root
	return *repo
}

// A Searcher defines the front-end search interface.
//
// A Search() is executed without any Repos being selected by
// the caller. The callee must determine which (if any) Repo
// are relevant to the request, and will likely make SearchRepo
// calls to retrieve the search results before returning a
// merged SearchResponse.
type Searcher interface {
	Search(request SearchRequest) (*SearchResponse, error)
}

// An Indexer can Index one or more Repos
//
// The Index call will not replace any Repo with a key matching that
// in the request.
type Indexer interface {
	Index(request IndexRequest) (*IndexResponse, error)
}

// An IndexRequest is sent when creating (indexing) a Repo.
// The value of the 'host' and 'port.rpc' keys of the Meta attribute
// are used by the indexer to determine whether to proxy the request
// to another afindd. Leaving the keys unpopulated or empty will
// cause indexing to happen on the local machine.
type IndexRequest struct {
	Key  string            `json:"key"`  // The Key for the new Repo
	Root string            `json:"root"` // The root path for all dirs
	Dirs []string          `json:"dirs"` // Sub directories of root to index
	Meta map[string]string `json:"meta"` // metadata applied to all new repos

	Recurse bool    `json:"-"`       // recursion is controlled locally
	Timeout float64 `json:"timeout"` // overrides the default request timeout
}

// The response to index calls. Contains details about the Repo
// indexed on the 'host' indicated in Repo.Meta
type IndexResponse struct {
	Repo    *Repo `json:"repo"`
	Elapsed time.Duration
	Error   string
}

func newIndexRequest(key, root string, dirs []string) IndexRequest {
	ir := IndexRequest{
		Key:  key,
		Root: root,
		Dirs: make([]string, len(dirs)),
		Meta: make(map[string]string),
	}
	for i, dir := range dirs {
		ir.Dirs[i] = dir
	}
	return ir
}

func newIndexResponse() *IndexResponse {
	return &IndexResponse{}
}

// A SearchRequest is the search request struct used by the client.
//
// If the client supplies one or more RepoKeys, only Repos matching
// those key(s) are searched, as few as zero. If RepoKeys is empty,
// all Repo are consulted. As each Repo is consulted, if Meta is
// supplied, each key's value is compared with the repo if it has
// a matching key. If all the request Meta fields match each repo
// in that fashion, the repo will be used for the search.
type SearchRequest struct {
	Re            string            `json:"re"`        // search regexp
	PathRe        string            `json:"path_re"`   // pathname regexp
	CaseSensitive bool              `json:"cs"`        // true=case-sensitive
	RepoKeys      []string          `json:"repo_keys"` // repos to search
	Meta          map[string]string `json:"meta"`      // repo metadata to match

	// Recursive query: set to have afindd search recursively one hop
	// Not honoured via JSON (recursion is controlled internally)
	Recurse bool    `json:"-"`
	Timeout float64 `json:"timeout"` // overrides the default request timeout
}

func newSearchRequest(re, pathRe string, cs bool, repoKeys []string) SearchRequest {
	req := SearchRequest{
		Re:            re,
		PathRe:        pathRe,
		CaseSensitive: cs,
		RepoKeys:      make([]string, len(repoKeys)),
	}
	for i, key := range repoKeys {
		req.RepoKeys[i] = key
	}
	return req
}

// The response struct used for a Repo Search.
//
// This response may represent the results of more than one discrete
// Repo search; that is, its results have been merged and ranked by
// the afind service
type SearchResponse struct {
	Files           map[string]map[string]map[string]string
	Errors          map[string]string // Error message per Repo
	NumLinesMatched int64             // total number of lines with 1 or more matches
	Elapsed         time.Duration     // Time elapsed performing the search
}

func newSearchResponse() *SearchResponse {
	return &SearchResponse{
		Files:  make(map[string]map[string]map[string]string),
		Errors: make(map[string]string),
	}
}

type SearchRepoResponse struct {
	Repo     *Repo
	Matches  map[string]map[string]string
	NumLines int // total number of lines scanned
	Error    string
}

func newSearchRepoResponse() *SearchRepoResponse {
	return &SearchRepoResponse{Matches: make(map[string]map[string]string)}
}

func newSearchRepoResponseFromError(err error) *SearchRepoResponse {
	return &SearchRepoResponse{
		Matches: make(map[string]map[string]string),
		Error:   err.Error(),
	}
}
