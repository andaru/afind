package afind

import (
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

func (b ByteSize) AsInt64() int64 {
	return int64(b)
}

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
	return fmt.Sprintf("%dB", b.AsInt64())
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
	Key       string            `json:"key"`        // Unique key
	IndexPath string            `json:"index_path"` // Path to the .afindex file covering this Repo
	Root      string            `json:"root"`       // Root path (under which Dirs are rooted)
	Meta      map[string]string `json:"meta"`       // User configurable metadata for this Repo
	State     RepoState         `json:"state"`      // Current repository indexing state

	// Metadata produced during indexing
	NumDirs   int      `json:"num_dirs"`          // Number of directories indexed
	NumFiles  int      `json:"num_files"`         // Number of files indexed
	SizeIndex ByteSize `json:"size_index,string"` // Size of index
	SizeData  ByteSize `json:"size_data,string"`  // Size of the source data
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

func newRepoFromIndexRequest(request *IndexRequest) *Repo {
	repo := newRepo(request.Key, "", request.Meta)
	repo.Root = request.Root
	return repo
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
type Indexer interface {
	Index(request IndexRequest) (*IndexResponse, error)
}

// An IndexRequest is the metadata for creating one or more Repo.
type IndexRequest struct {
	Key  string
	Root string
	Dirs []string
	Meta map[string]string // metadata applied to all new repos
}

// The response to calls to Indexer.Index.
type IndexResponse struct {
	Repo    *Repo
	Elapsed time.Duration
	// todo: errors
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

func newIndexRequestWithMeta(key, root string, dirs []string,
	meta map[string]string) IndexRequest {

	ir := newIndexRequest(key, root, dirs)
	for k, v := range meta {
		ir.Meta[k] = v
	}
	return ir
}

func newIndexResponse() *IndexResponse {
	return &IndexResponse{}
}

// A SearchRequest is the client request struct.
//
// If the user supplies one or more RepoKeys, only Repos matching those
// key(s) are searched. If RepoKeys is empty all Repo are consulted.
type SearchRequest struct {
	Re, PathRe    string
	CaseSensitive bool
	RepoKeys      []string
	Meta          map[string]string
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
