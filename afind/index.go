package afind

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/utils"
	"github.com/andaru/afind/walkablefs"
	"github.com/andaru/codesearch/index"
	"github.com/savaki/par"
	"golang.org/x/tools/godoc/vfs"
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
	Key   string   `json:"key"`  // The Key for the new Repo
	Root  string   `json:"root"` // The root path for all Dirs
	Dirs  []string `json:"dirs"` // Sub directories of Root to index
	Files []string // Individual files to index. No impl, so not yet JSON tagged
	Meta  Meta     `json:"meta"` // Metadata set on the Repo

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
	Repo  *Repo             `json:"repo"`
	Error *errs.StructError `json:"error,omitempty"`
}

const (
	maxShards = 32
)

// Sets the error string on the IndexResult if the error passed is not
// nil, else is a no-op.
func (ir *IndexResult) SetError(err error) {
	if e, ok := err.(*errs.StructError); ok && e != nil {
		ir.Error = e
	} else if err != nil {
		ir.Error = errs.NewStructError(err)
	}
}

// Creates a new, keyed but otherwise empty IndexQuery.
// There must be at least one entry in Dirs or Files when the
// query is sent.
func NewIndexQuery(key string) IndexQuery {
	return IndexQuery{
		Key:   key,
		Files: make([]string, 0),
		Dirs:  make([]string, 0),
		Meta:  make(Meta),
	}
}

// Normalize validates and moralizes the IndexQuery
func (r *IndexQuery) Normalize() error {
	// Validate
	if r.Key == "" {
		return errs.NewValueError("key", "Key must not be empty")
	} else if len(r.Dirs) == 0 {
		return errs.NewValueError(
			"dirs",
			"Must provide one one more sub dirs (such as [`.`])")
	} else if !path.IsAbs(r.Root) {
		return errs.NewValueError(
			"root", "Root must be an absolute path")
	}
	// Confirm all sub directories provided are not absolute, and remove
	// any duplicate paths to avoid duplicate indexing of files.
	seen := map[string]struct{}{}
	for _, dir := range r.Dirs {
		if path.IsAbs(dir) {
			return errs.NewValueError(
				"dirs", "Dirs must not be absolute paths")
		}
		// We require a relative path, but FileSystem uses a rooted
		// path, so '.' needs to be seen as '/'.
		if dir == "." {
			dir = "/"
		}
		seen[dir] = struct{}{}
	}
	dirs := []string{}
	for dir, _ := range seen {
		dirs = append(dirs, dir)
	}
	r.Dirs = dirs
	return nil
}

// Returns a pointer to a new indexing Result
func NewIndexResult() *IndexResult {
	return &IndexResult{}
}

// Indexer implementation

// indexer carries our private Indexer implementation
type indexer struct {
	cfg    *Config
	shards []index.IndexWriter
	root   string // normalized root directory
	repos  KeyValueStorer
	writer index.IndexWriter
	names  chan string
}

// NewIndexer returns a new Indexer implementation given a
// configuration and repo store
func NewIndexer(
	cfg *Config,
	repos KeyValueStorer) indexer {

	return indexer{
		cfg:    cfg,
		shards: []index.IndexWriter{},
		repos:  repos,
		names:  make(chan string),
	}
}

func getRoot(c *Config, q *IndexQuery) string {
	q.Root = strings.TrimSuffix(q.Root, string(os.PathSeparator))
	if c.IndexInRepo {
		return q.Root
	}
	return path.Join(c.IndexRoot, q.Key)
}

func getIndexWriter(ctx context.Context, name string) (index.IndexWriter, error) {
	if f := ctx.Value("IndexWriterFunc"); f != nil {
		return f.(func(name string) index.IndexWriter)(name), nil
	}
	// Use the real implementation as the default IndexWriterFunc.
	// Create will log.Fatal if name doesn't exist, so try to create
	// the path first.
	if err := os.MkdirAll(path.Dir(name), 0755); err != nil && !os.IsExist(err) {
		return nil, nil
	}
	return index.Create(name), nil
}

func getFileSystem(ctx context.Context, root string) walkablefs.WalkableFileSystem {
	if fs := ctx.Value("FileSystem"); fs != nil {
		return fs.(walkablefs.WalkableFileSystem)
	}
	// default to a walkable local OS filesystem at the root dir
	return walkablefs.New(vfs.OS(root))
}

func shardName(key string, n int) string {
	return key + "-" + strconv.Itoa(n) + ".afindex"
}

// Index executes the indexing request (on this machine, in this
// case), producing a response and optionally an error.
func (i indexer) Index(ctx context.Context, req IndexQuery) (
	resp *IndexResult, err error) {

	log.Info("index [%v]", req.Key)
	start := time.Now()
	// Setup the response
	resp = NewIndexResult()

	if err = req.Normalize(); err != nil {
		log.Info("index [%v] error: %v", req.Key, err)
		resp.Error = errs.NewStructError(err)
		return
	}

	// create index shards
	var nshards int
	if nshards = i.cfg.NumShards; nshards == 0 {
		nshards = 1
	}
	nshards = utils.MinInt(nshards, maxShards)
	i.shards = make([]index.IndexWriter, nshards)
	i.root = getRoot(i.cfg, &req)

	for n := range i.shards {
		name := path.Join(i.root, shardName(req.Key, n))
		if ixw, err := getIndexWriter(ctx, name); err != nil {
			resp.Error = errs.NewStructError(err)
			return resp, nil
		} else {
			i.shards[n] = ixw
		}
	}

	fs := getFileSystem(ctx, i.root)
	repo := newRepoFromQuery(&req, i.root)
	repo.SetMeta(i.cfg.RepoMeta, req.Meta)
	resp.Repo = repo

	// Add query Files and scan Dirs for files to index
	names, err := i.scanner(fs, &req)
	ch := make(chan int, nshards)
	chnames := make(chan string, 100)
	go func() {
		for _, name := range names {
			chnames <- name
		}
		close(chnames)
	}()
	reqch := make(chan par.RequestFunc, nshards)
	for _, shard := range i.shards {
		reqch <- indexShard(&i, &req, shard, fs, chnames, ch)
	}
	close(reqch)
	err = par.Requests(reqch).WithConcurrency(nshards).DoWithContext(ctx)
	close(ch)

	// Await results, each indicating the number of files scanned
	for num := range ch {
		repo.NumFiles += num
	}

	repo.NumShards = len(i.shards)
	// Flush our index shard files
	for _, shard := range i.shards {
		shard.Flush()
		repo.SizeIndex += ByteSize(shard.IndexBytes())
		repo.SizeData += ByteSize(shard.DataBytes())
		log.Debug("index flush %v (data) %v (index)",
			repo.SizeData, repo.SizeIndex)
	}
	repo.ElapsedIndexing = time.Since(start)
	repo.TimeUpdated = time.Now().UTC()

	var msg string
	if err != nil {
		repo.State = ERROR
		resp.SetError(err)
		msg = "error: " + resp.Error.Error()
	} else {
		repo.State = OK
		msg = "ok " + fmt.Sprintf(
			"(%v files, %v data, %v index)",
			repo.NumFiles, repo.SizeData, repo.SizeIndex)
	}
	log.Info("index [%v] %v [%v]", req.Key, msg, repo.ElapsedIndexing)
	return
}

func trimLeadingSlash(name string) string {
	return strings.TrimPrefix(name, string(os.PathSeparator))
}

// The scanner returns files eligible for indexing
func (i *indexer) scanner(fs walkablefs.WalkableFileSystem, query *IndexQuery) ([]string, error) {
	var err error

	names := make([]string, 0)

	// First, add any specific files in the request
	for _, name := range query.Files {
		// Only add files that we can stat to the list
		if fi, err := fs.Lstat(trimLeadingSlash(name)); err == nil && !fi.IsDir() {
			names = append(names, name)
		}
	}

	// For each of the Dirs, walk the contents
	for _, path := range query.Dirs {
		walker := func(p string, info os.FileInfo, werr error) error {
			if werr != nil {
				return werr
			} else if info == nil {
				return nil
			} else if IndexPathExcludes.MatchFile(p) {
				// Skip excluded extensions and dirs
				if info.IsDir() {
					return filepath.SkipDir
				}
			} else if !info.IsDir() && info.Mode()&os.ModeType == 0 {
				names = append(names, trimLeadingSlash(p))
			}
			return nil
		}
		err = fs.Walk(path, walker)
	}
	return names, err
}

func indexShard(
	i *indexer,
	q *IndexQuery,
	writer index.IndexWriter,
	fs walkablefs.WalkableFileSystem,
	in chan string,
	out chan int) par.RequestFunc {

	// While there are files to add, add them to the specified shard.
	return func(ctx context.Context) error {
		numFiles := 0
		for name := range in {
			r, err := fs.Open(name)
			if err == nil {
				writer.Add(name, r)
				_ = r.Close()
				numFiles++
			}
		}
		out <- numFiles
		return nil
	}

}
