package afind

import (
	"path"
)

// A Repo represents a single indexed repository of source code.
type Repo struct {
	Key      string            // Unique key
	UriIndex string            // URI of Repo's index
	Meta     map[string]string // User configurable metadata for this Repo
	State    RepoState

	RepoMeta
}

type RepoState int

// RepoState constants: the current state of a Repo
const (
	NULL     = iota
	INDEXING // currently indexing
	OK       // available for searching
	ERROR    // not available for searching
)

func newRepo(key, uriIndex string, meta map[string]string) *Repo {
	r := &Repo{
		Key:      key,
		UriIndex: uriIndex,
		Meta:     make(map[string]string),
	}
	for k, v := range meta {
		r.Meta[k] = v
	}
	return r
}

func newRepoFromIndexRequest(request IndexRequest) *Repo {
	return newRepo(request.Key, request.UriIndex(), request.Meta)
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

// Composition of interfaces to form the major parts of the system.

// An IndexRequest is the metadata for creating one or more Repo.
type IndexRequest struct {
	Key  string
	Root string
	Dirs []string
	Meta map[string]string // metadata applied to all new repos
}

func (req *IndexRequest) UriIndex() string {
	return path.Join(config.IndexRoot, "ix_"+req.Key+".afindex")
}

// Indexing statistics for a repo. Generated during Index() calls.
type RepoMeta struct {
	NumDirs   int     // Number of directories indexed
	NumFiles  int     // Number of files indexed
	Elapsed   float64 // Wallclock indexing time in seconds
	SizeIndex uint32  // Size of the source index file in bytes
	SizeData  int64   // Size of the data indexed by the Repo in bytes
}

// The union of the Repo and RepoMeta types.
// Returned in the IndexResponse by Indexer.Index().
type repoPlusStats struct {
	*Repo
	*RepoMeta
}

// The response to calls to Indexer.Index.
type IndexResponse struct {
	Repos map[string]*Repo
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
	return &IndexResponse{Repos: make(map[string]*Repo)}
}

// A SearchRequest is the client request struct.
//
// If the user supplies one or more RepoKeys, only Repos matching those
// key(s) are searched. If RepoKeys is empty all Repo are consulted.
type SearchRequest struct {
	Re, PathRe    string
	CaseSensitive bool
	RepoKeys      []string
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
	Files map[string]map[string]map[string]string

	NumLinesMatched int64 // total number of lines with 1 or more matches
	ElapsedNs       int64 // nanoseconds elapsed during search(es)

	// todo: errors
}

type SearchRepoResponse struct {
	Repo     *Repo
	Matches  map[string]map[string]string
	NumLines int // total number of lines scanned
}

func newSearchRepoResponse() *SearchRepoResponse {
	return &SearchRepoResponse{Matches: make(map[string]map[string]string)}
}

func newSearchResponse() *SearchResponse {
	return &SearchResponse{
		Files:           make(map[string]map[string]map[string]string),
		ElapsedNs:       0,
		NumLinesMatched: 0,
	}
}
