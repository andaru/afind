package afind

import (
	"os"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/walkablefs"
)

// Searcher provides a generic search interface.
// An implementation may restrict a search to being
// run on subset of repositories, such as one on
// a backend, allowing the front-end to compose
// a search query appropriately.
type Searcher interface {
	Search(context.Context, SearchQuery) (*SearchResult, error)
}

type ChanSearcher interface {
	Search(ctx context.Context, input <-chan SearchQuery, output chan<- *SearchResult) error
}

// SearchFunc is the generic backend search function prototype
type SearchFunc func(SearchQuery, chan *SearchResult) error

// A client Search Query.
//
// If one or more RepoKeys are supplied, only Repos matching those
// key(s) are searched, if any. If RepoKeys is empty, all Repo are
// consulted which match any metadata supplied. Re is the regular
// expression to match in the final repositories. PathRe is a regular
// expression matched against names of files with matches to Re.
type SearchQuery struct {
	// Major search query attributes

	// search regular expression
	Re string `json:"re"`
	// a regular expression to match filenames with search matches
	PathRe string `json:"path_re"`
	// if true, perform case insensitive searches
	IgnoreCase bool `json:"i"`

	// Repository filtering attributes
	// Search only these repositories if not empty
	RepoKeys []string `json:"repo_keys"`

	// Metadata to match against all repos if RepoKeys empty
	Meta Meta `json:"meta"`
	// If true, use regular expressions to match Meta
	MetaRegexpMatch bool `json:"meta_use_regexp,omitempty"`

	// Search context (number of lines ahead, behind or both)
	Context SearchContext `json:"context"`

	// Maximum number of matches to return
	MaxMatches uint64 `json:"max_matches"`

	// Override the 30 second default request timeout
	Timeout time.Duration `json:"timeout"`

	// recursive query: set on RPC requests to enable single hop
	// recursion. JSON requests may not explicitly set recursion
	// as an extra loop safety. The HTTP request handler sets
	// recursion appropriately.
	Recurse bool `json:"-"`
}

// SearchContext provides options around the lines of context
// surrounding the match text. By default, no context lines are
// supplied.
type SearchContext struct {
	Both int `json:"both"`
	Pre  int `json:"pre"`
	Post int `json:"post"`
}

// Returns a SearchQuery value given the parameters
func NewSearchQuery(re, pathRe string, ignore bool, repoKeys []string) SearchQuery {
	return SearchQuery{
		Re:         re,
		PathRe:     pathRe,
		IgnoreCase: ignore,
		RepoKeys:   repoKeys,
		Meta:       *new(Meta),
	}
}

func (q *SearchQuery) firstKey() string {
	if len(q.RepoKeys) > 0 {
		return q.RepoKeys[0]
	}
	return ""
}

type fileMap map[string]string

type repoMatchMap map[string]fileMap

type matchMap map[string]repoMatchMap

// A search Result.
// The same SearchResult struct is used throughout the searcher so that
// pipelines and context can be used to easily manage and merge
// concurrent results. The results are unranked.
type SearchResult struct {
	// Matches per file, repo key and line number to text of lines matching
	Matches map[string]map[string]map[string]string `json:"matches"`

	Errors      map[string]*errs.StructError `json:"errors,omitempty"` // Per repo errors
	Error       string                       `json:"error,omitempty"`  // Any global error
	NumMatches  uint64                       `json:"num_matches"`      // Search hit count
	Elapsed     time.Duration                `json:"elapsed_total"`    // Whole search time
	ElapsedPost time.Duration                `json:"elapsed_posting"`  // Posting query time
	Repos       map[string]*Repo             `json:"repos,omitempty"`  // Repository details
	MaxMatches  uint64                       `json:"max_matches"`      // Max matches requested
}

// Returns a pointer to an initialized search Result.
func NewSearchResult() *SearchResult {
	return &SearchResult{
		Matches: make(map[string]map[string]map[string]string),
		Errors:  make(map[string]*errs.StructError),
		Repos:   make(map[string]*Repo),
	}
}

// Sets the error string on the SearchResult if the error passed is not
// nil, else is a no-op.
func (sr *SearchResult) SetError(err error) {
	if err != nil {
		sr.Error = err.Error()
	}
}

// Updates the SearchResult from the contents of other
func (r *SearchResult) Update(other *SearchResult) {
	enough := false
	for file, rmatches := range other.Matches {
		if enough {
			break
		} else if len(rmatches) == 0 {
			continue
		}
		for repo, matches := range rmatches {
			if len(matches) == 0 {
				continue
			}
			if enough = !r.addFileRepoMatches(file, repo, matches); enough {
				break
			}
		}
	}
	// Copy errors and repository information
	for k, v := range other.Errors {
		r.Errors[k] = v
	}
	for k, v := range other.Repos {
		r.Repos[k] = v
	}

	// Append unique messages to the global error string
	if r.Error == "" {
		r.Error = other.Error
	} else if other.Error != "" && !strings.HasSuffix(r.Error, other.Error) {
		r.Error += "\n" + other.Error
	}

	r.Elapsed += other.Elapsed
	r.ElapsedPost += other.ElapsedPost
}

func (r *SearchResult) addFileRepoMatches(fname, repokey string, matches fileMap) bool {
	if _, ok := r.Matches[fname]; !ok {
		r.Matches[fname] = make(map[string]map[string]string)
	}
	if _, ok := r.Matches[fname][repokey]; !ok {
		r.Matches[fname][repokey] = make(fileMap)
	}

	for k, v := range matches {
		r.Matches[fname][repokey][k] = v
		r.NumMatches++
		// Stop if we've hit the match limit
		if r.MaxMatches > 0 && r.NumMatches >= r.MaxMatches {
			return false
		}
	}
	return true
}

// The Searcher implementation
type searcher struct {
	cfg   *Config
	repos KeyValueStorer
	fs    walkablefs.WalkableFileSystem
}

// Returns a new value of our Searcher implementation
func NewSearcher(cfg *Config, repos KeyValueStorer) searcher {
	return searcher{
		cfg:   cfg,
		repos: repos,
	}
}

// Search performs the supplied request, returning
// an initialized search response and an error.
//
// This particular searcher only allows searches on individual
// repositories. For that reason, this function panics if passed a
// request that does not have exactly one key in the RepoKeys
// attribute. This forces multi-repository searches to be composed
// using other libraries.
func (s searcher) Search(ctx context.Context, query SearchQuery) (
	*SearchResult, error) {

	start := time.Now()
	resp := NewSearchResult()
	// If the repo is unavailable but not due to indexing,
	// exit with RepoUnavailableError.
	repokey := query.firstKey()

	irepo := s.repos.Get(repokey)
	if irepo == nil {
		resp.Errors[repokey] = errs.NewStructError(
			errs.NewRepoUnavailableError())
		return resp, nil
	}

	repo := irepo.(*Repo)
	resp.Repos[repo.Key] = repo
	if repo.State == INDEXING {
		return resp, nil
	} else if repo.State != OK {
		resp.Errors[repokey] = errs.NewStructError(errs.NewRepoUnavailableError())
		return resp, nil
	}

	shards := repo.Shards()
	left := len(shards)
	ch := make(chan *SearchResult, 1)
	defer close(ch)

	for _, shard := range shards {
		go func(r *Repo, fname string) {
			sr, err := searchLocal(ctx, query, r, fname)
			if err != nil {
				// Maybe mark the repo as errored (unavailable)
				if os.IsNotExist(err) || os.IsPermission(err) {
					r.State = ERROR
					_ = s.repos.Set(r.Key, r)
				}
				sr.Error = err.Error()
			}
			select {
			case <-ctx.Done():
				return
			default:
				ch <- sr
			}
		}(repo, shard)
	}

	// Collect and merge errors
	var err error
	for left > 0 {
		select {
		case in := <-ch:
			resp.Update(in)
			left--
			if resp.MaxMatches > 0 && resp.NumMatches >= resp.MaxMatches {
				left = 0
			}
		case <-ctx.Done():
			err = errs.NewTimeoutError("search")
			resp.Error = err.Error()
			left = 0
		}
	}

	elapsed := time.Since(start)
	resp.Elapsed = elapsed
	return resp, err
}

// search an individiaul afindex search for the repo for the request
func searchLocal(ctx context.Context, req SearchQuery, repo *Repo, fname string) (
	resp *SearchResult, err error) {
	return newGrep(fname, repo.Root, getFileSystem(ctx, repo.Root)).search(ctx, req)
}
