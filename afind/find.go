package afind

import (
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/stopwatch"
	"github.com/andaru/codesearch/index"
	"github.com/andaru/codesearch/regexp"
	"github.com/savaki/par"
)

// The find API endpoint is intended for use as an autocomplete filler.

// Due to the rapid nature of such requests, default timeouts are set
// very low.

// Finder searches Repo posting indices for all files matching
// the query.
type Finder interface {
	Find(context.Context, FindQuery) (*FindResult, error)
}

// FindQuery contains the query and query options for passing to Finder
type FindQuery struct {
	// The path regular expression
	PathRe string `json:"path_re"`
	// if true, perform case insensitive searches
	IgnoreCase bool `json:"i"`

	// Repository filtering attributes
	// Search only these repositories if not empty
	RepoKeys []string `json:"repo_keys,omitempty"`

	// Metadata to match against all repos if RepoKeys empty
	Meta Meta `json:"meta"`
	// If true, use regular expressions to match Meta
	MetaRegexpMatch bool `json:"meta_use_regexp,omitempty"`

	// Maximum number of files to return
	MaxMatches uint64 `json:"max_matches"`

	// Recursion flag, as per IndexQuery/SearchQuery
	Recurse bool `json:"-"`

	// Overrides the default timeout
	Timeout time.Duration `json:"timeout"`
}

// NewFindQuery returns a pointer to an initialized FindQuery
func NewFindQuery() FindQuery {
	return FindQuery{
		RepoKeys: []string{},
		Meta:     Meta{},
	}
}

func (q *FindQuery) firstKey() string {
	if len(q.RepoKeys) > 0 {
		return q.RepoKeys[0]
	}
	return ""
}

// FindResult contains the results corresponding for a FindQuery
type FindResult struct {
	Matches map[string]map[string]int    `json:"matches"`
	Errors  map[string]*errs.StructError `json:"errors,omitempty"`

	// A global error that occured prior to executing any search query.
	Error *errs.StructError `json:"error,omitempty"`
	// The number of results received across all files
	NumMatches uint64 `json:"num_matches"`
	// Maximum number of files to return
	MaxMatches uint64 `json:"max_matches"`
}

// NewFindResult returns a pointer to an initialized FindResult
func NewFindResult() *FindResult {
	return &FindResult{
		Matches: map[string]map[string]int{},
		Errors:  map[string]*errs.StructError{},
	}
}

// EnoughrResults returns true if sufficient files have been found in matches
func (fr FindResult) EnoughResults() bool {
	return fr.MaxMatches > 0 && fr.NumMatches >= fr.MaxMatches
}

// SetError sets the Error attribute appropriately
func (fr *FindResult) SetError(err error) {
	if e, ok := err.(*errs.StructError); ok {
		fr.Error = e
	} else if err != nil {
		fr.Error = errs.NewStructError(err)
	}
}

// Update merges the contents of other
func (fr *FindResult) Update(other *FindResult) {
	for k, v := range other.Errors {
		fr.Errors[k] = v
	}
	for file, repos := range other.Matches {
		if _, ok := fr.Matches[file]; !ok {
			fr.Matches[file] = map[string]int{}
		}
		for repo := range repos {
			fr.Matches[file][repo]++
		}
	}
	fr.NumMatches += other.NumMatches
}

// The Finder implementation
type finder struct {
	cfg   *Config
	repos KeyValueStorer
}

// NewFinder returns a new value of our Finder implementation
func NewFinder(cfg *Config, repos KeyValueStorer) Finder {
	return finder{
		cfg:   cfg,
		repos: repos,
	}
}

var (
	regexpAll = mustCompile(".")
)

func mustCompile(pattern string) *regexp.Regexp {
	cre, err := regexp.Compile(pattern)
	if err == nil {
		return cre
	}
	panic("regexp did not compile: " + err.Error())
}

func (f finder) Find(ctx context.Context, query FindQuery) (fr *FindResult, err error) {
	var reg *regexp.Regexp

	log.Info("find [%s] keys %v", query.PathRe, query.RepoKeys)
	sw := stopwatch.New()
	sw.Start("*")

	fr = NewFindResult()
	chQuery := make(chan par.RequestFunc, 100)
	chResult := make(chan fnamerepo, 10)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if len(query.RepoKeys) < 1 {
		fr.Error = kRepoKeyEmptyError
		goto done
	}

	if query.PathRe == "" {
		fr.Error = errs.NewStructError(
			errs.NewInvalidRequestError("path_re must not be empty"))
		goto done
	}

	if query.IgnoreCase {
		query.PathRe = "(?i)" + query.PathRe
	}

	reg, err = regexp.Compile(query.PathRe)
	if err != nil {
		fr.Error = errs.NewStructError(err)
		goto done
	}

	// concurrently perform queries across all mentioned repo

	for _, key := range query.RepoKeys {
		if v := f.repos.Get(key); v == nil {
			fr.Errors[key] = kRepoUnavailableError
		} else {
			repo := v.(*Repo)
			for _, fn := range repo.Shards() {
				chQuery <- shardFind(fn, repo.Key, reg, chResult)
			}
		}

	}
	close(chQuery)
	// await and merge results
	sw.Start("awaitResults")
	go func() {
		_ = par.Requests(chQuery).WithConcurrency(4).DoWithContext(ctx)
		close(chResult)
	}()

	for in := range chResult {
		if _, ok := fr.Matches[in.fname]; !ok {
			fr.Matches[in.fname] = map[string]int{}
		}
		fr.Matches[in.fname][in.repo]++
		fr.NumMatches++
	}
done:
	elapsed := sw.Stop("*")
	if len(fr.Matches) > 0 {
		log.Info("find [%v] done (%d matches) (%v)",
			query.PathRe, len(fr.Matches), elapsed)
	}
	return
}

type fnamerepo struct {
	fname string
	repo  string
}

func shardFind(fn string, key string, re *regexp.Regexp, results chan fnamerepo) par.RequestFunc {
	return func(ctx context.Context) (err error) {
		ix, err := index.Open(fn)
		if err != nil {
			return
		}
		q := index.RegexpQuery(regexpAll.Syntax)
		post := ix.PostingQuery(q)
		for _, id := range post {
			name := ix.Name(id)
			if re.MatchString(name, true, true) < 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				results <- fnamerepo{name, key}
			}
		}
		return
	}
}

// Normalize validates and moralizes the IndexQuery
func (q *FindQuery) Normalize() error {
	// Validate
	if len(q.PathRe) < 3 {
		return errs.NewValueError("path_re", "must be at least 3 characters")
	}
	return nil
}
