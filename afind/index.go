package afind

import (
	"path"
	"path/filepath"
	"time"

	"golang.org/x/net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/codesearch/index"
	"os"
	"strconv"
)

// An Indexer can index text sources for a single Repo each request
type Indexer interface {
	// Indexes sources defined by the request. If indexing is
	// successful, a Repo is stored in the system database.
	//
	// If a Repo with the same Key as the request exists, an error
	// is returned along with the existing Repo in the response
	// (to allow backends to cache-fill frontends).
	Index(context.Context, IndexQuery) (*IndexResult, error)
}

// An IndexQuery is sent when calling Indexer.Index().
//
// The value of the 'host' key of the Meta attribute is used by the
// Indexer to determine whether to proxy the request to an afindd task
// over a socket. The default value for 'host' is obtained from
// the request context's config (from RepoMeta["host"])
type IndexQuery struct {
	Key  string   `json:"key"`  // The Key for the new Repo
	Root string   `json:"root"` // The root path for all Dirs
	Dirs []string `json:"dirs"` // Sub directories of Root to index
	Meta Meta     `json:"meta"` // Metadata set on the Repo

	// Recursive query: set to have afindd search recursively one hop
	// JSON payloads cannot set recursion (the HTTP request handler
	// sets recursion appropriately).
	Recurse bool          `json:"-"`       // recursion is controlled locally
	Timeout time.Duration `json:"timeout"` // overrides the default request timeout
}

// The response to Indexer.Index() method calls.
//
// Contains details about the indexing call that just completed.
// If Error is not empty, an error occured, else the request was
// successful.
type IndexResult struct {
	Repo  *Repo  `json:"repo"`
	Error string `json:"error,omitempty"`
}

// Sets the error string on the IndexResult if the error passed is not
// nil, else is a no-op.
func (ir *IndexResult) SetError(err error) {
	if err != nil {
		ir.Error = err.Error()
	}
}

// Creates a new IndexQuery given a key, index root directory and a
// slice of sub directories to be indexed.
func NewIndexQuery(key, root string, dirs []string) IndexQuery {
	ir := IndexQuery{
		Key:  key,
		Root: root,
		Dirs: make([]string, len(dirs)),
		Meta: make(Meta),
	}
	for i, dir := range dirs {
		ir.Dirs[i] = dir
	}
	return ir
}

// Validates the IndexQuery, returning the first error found
func (r *IndexQuery) ValidationError() error {
	if len(r.Dirs) == 0 {
		return errs.NewValueError(
			"dirs",
			"Must provide one one more sub dirs (such as [`.`])")
	} else if !path.IsAbs(r.Root) {
		return errs.NewValueError(
			"root", "Root must be an absolute path")
	}
	// Confirm all sub directories provided are not absolute
	for _, dir := range r.Dirs {
		if path.IsAbs(dir) {
			return errs.NewValueError(
				"dirs", "Dirs must not be absolute paths")
		}
	}
	return nil
}

// Returns a pointer to a new indexing Result
func NewIndexResult() *IndexResult {
	return &IndexResult{}
}

// Indexer implementation

// indexer carries our private Indexer implementation
type indexer struct {
	cfg   *Config
	repos KeyValueStorer
}

// NewIndexer returns a new Indexer implementation given a
// configuration and repo store
func NewIndexer(cfg *Config, repos KeyValueStorer) indexer {
	return indexer{cfg: cfg, repos: repos}
}

func getroot(c *Config, q *IndexQuery) string {
	if c.IndexInRepo {
		return q.Root
	}
	return path.Join(c.IndexRoot, q.Key)
}

// Index executes the request, producing a response and optionally an error
func (i indexer) Index(ctx context.Context, req IndexQuery) (
	resp *IndexResult, err error) {

	log.Info("index [%v]", req.Key)

	start := time.Now()
	resp = NewIndexResult()

	// do request validation and exit early on errors
	if err = req.ValidationError(); err != nil {
		return
	}

	// If the Repo exists, just return it. This behavior is used
	// to backfill repos on a front end (e.g., if it just
	// restarted).
	if r := i.repos.Get(req.Key); r != nil {
		resp.Repo = r.(*Repo)
		return
	}

	// By default, use 2 shards (in e.g., tests)
	numShards := i.cfg.NumShards
	if numShards == 0 {
		numShards = 2
	}

	indexroot := getroot(i.cfg, &req)
	if err = makeIndexRoot(indexroot); err != nil && !os.IsExist(err) {
		return
	}

	repo := newRepoFromQuery(&req, indexroot)
	repo.SetMeta(i.cfg.RepoMeta, req.Meta)
	resp.Repo = repo

	// create index shards
	shards := make([]*index.IndexWriter, numShards)
	for n := range shards {
		name := path.Join(repo.IndexPath,
			req.Key+"-"+strconv.Itoa(n)+".afindex")
		log.Debug("adding shard [%d] %v", n, name)
		shards[n] = index.Create(name)
	}
	ndirs := 0
	nfiles := 0
	advance := len(repo.Root) + 1
	nshards := len(shards)

	// For each provided subdir, walk the contents in series for now
	for _, path := range req.Dirs {
		// check to see if the context's timeout has elapsed
		select {
		case <-ctx.Done():
			// set the timeout error and stop working
			resp.SetError(errs.NewTimeoutError("index"))
			return
		default:
			// pass through if not done
		}

		path = filepath.Join(req.Root, path)
		log.Debug("walking subdir %v", path)
		err = filepath.Walk(path,
			func(p string, info os.FileInfo, werr error) error {
				// Track the last walk error if set, then bail
				if werr != nil {
					err = werr
					return nil
				}
				// Skip entries without info
				if info == nil {
					return nil
				}
				// Skip excluded extensions and prefixes
				if IndexPathExcludes.MatchFile(p) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				if info.IsDir() {
					ndirs++
				} else if info.Mode()&os.ModeType == 0 {
					// Add the file to the next shard
					nfiles++
					slotnum := nfiles % nshards
					shards[slotnum].AddFileInRoot(
						repo.Root, p[advance:])
				}
				return nil
			})
	}

	// Update counters
	repo.NumFiles = nfiles
	repo.NumDirs = ndirs
	repo.NumShards = nshards
	// Flush our index shard files
	for _, shard := range shards {
		shard.Flush()
		repo.SizeData += ByteSize(shard.DataBytes())
		repo.SizeIndex += ByteSize(shard.IndexBytes())
	}
	repo.ElapsedIndexing = time.Since(start)
	repo.TimeCreated = time.Now().UTC()

	var msg string
	if err != nil {
		repo.State = ERROR
		resp.Error = err.Error()
		msg = "failed with error: " + resp.Error
	} else {
		repo.State = OK
		msg = "suceeded"
	}
	log.Info("index backend [%v] %v [%v]", req.Key, msg, repo.ElapsedIndexing)
	return
}

func makeIndexRoot(path string) error {
	return os.MkdirAll(path, 0755)
}
