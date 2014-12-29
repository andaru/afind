package afind

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/utils"
	"github.com/andaru/codesearch/index"
	"github.com/savaki/par"
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
	Repo  *Repo  `json:"repo"`
	Error string `json:"error,omitempty"`
}

const (
	maxShards = 32
)

// Sets the error string on the IndexResult if the error passed is not
// nil, else is a no-op.
func (ir *IndexResult) SetError(err error) {
	if err != nil {
		ir.Error = err.Error()
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
	cfg     *Config
	shards  []index.IndexWriter
	repos   KeyValueStorer
	sourcer readerSourcer
	writer  index.IndexWriter // template, used for shards
	names   chan string
}

// NewIndexer returns a new Indexer implementation given a
// configuration and repo store
func NewIndexer(cfg *Config, repos KeyValueStorer) indexer {
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

type readerSourcer interface {
	Reader(name string) (io.ReadCloser, error)
}

type indexWriter interface {
	Create(name string) index.IndexWriter
	Add(name string, r io.Reader)
}

type localReader struct {
	root string
}

func (l localReader) Reader(name string) (io.ReadCloser, error) {
	if l.root == "" {
		panic("localReader.root must not be empty")
	}
	return os.Open(path.Join(l.root, name))
}

func getReaderSourcer(ctx context.Context, root string) readerSourcer {
	// Allow the readerSourcer to be swapped in for testing.
	// Defaults to using a local filesystem reader rooted at the
	// request Root path, which has been validated above.
	if _sourcer := ctx.Value("readerSourcer"); _sourcer != nil {
		return _sourcer.(readerSourcer)
	}
	return localReader{root}
}

func getIndexWriter(ctx context.Context) index.IndexWriter {
	if _writer := ctx.Value("IndexWriter"); _writer != nil {
		return _writer.(index.IndexWriter)
	}
	return nil
}

func shardName(key string, n int) string {
	return key + "-" + strconv.Itoa(n) + ".afindex"
}

func (i *indexer) makeIndexShards(key string) {
	// create index shards
	numShards := i.cfg.NumShards
	if numShards == 0 {
		numShards = 2
	}
	numShards = utils.MinInt(numShards, maxShards)
	i.shards = make([]index.IndexWriter, numShards)
	log.Debug("index [%v] has %d shards", key, len(i.shards))
	for n := range i.shards {
		name := shardName(key, n)
		if i.writer == nil {
			i.shards[n] = index.Create(name)
		} else {
			i.shards[n] = i.writer.Create(name)
		}
	}
}

// Index executes the indexing request (on this machine, in this
// case), producing a response and optionally an error.
func (i indexer) Index(ctx context.Context, req IndexQuery) (
	resp *IndexResult, err error) {

	log.Info("index [%v]", req.Key)
	start := time.Now()

	if err = req.Normalize(); err != nil {
		log.Info("index [%v] error: %v [%v]", req.Key, err, time.Since(start))
		return
	}

	root := getRoot(i.cfg, &req)
	if err = os.MkdirAll(root, 0755); err != nil && !os.IsExist(err) {
		return
	}

	// Setup the response
	resp = NewIndexResult()
	repo := newRepoFromQuery(&req, root)
	repo.SetMeta(i.cfg.RepoMeta, req.Meta)
	resp.Repo = repo
	i.sourcer = getReaderSourcer(ctx, req.Root)
	i.writer = getIndexWriter(ctx)

	// create index shards and concurrently perform indexing
	i.makeIndexShards(req.Key)
	// Add query Files and scan Dirs for files to index
	names, err := i.scanner(ctx, &req)
	nshards := len(i.shards)
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
		reqch <- indexShard(&i, &req, shard, chnames, ch)
	}
	close(reqch)
	err = par.Requests(reqch).WithConcurrency(nshards).DoWithContext(ctx)
	close(ch)

	// Await incoming results and update the response
	for num := range ch {
		repo.NumFiles += num
	}
	repo.NumShards = len(i.shards)
	// Flush our index shard files
	for _, shard := range i.shards {
		shard.Flush()
		repo.SizeData += ByteSize(shard.DataBytes())
		repo.SizeIndex += ByteSize(shard.IndexBytes())
	}
	repo.ElapsedIndexing = time.Since(start)
	repo.TimeUpdated = time.Now().UTC()

	var msg string
	if err != nil {
		repo.State = ERROR
		resp.Error = err.Error()
		msg = "error: " + resp.Error
	} else {
		repo.State = OK
		msg = "ok"
	}
	log.Info("index [%v] %v [%v]", req.Key, msg, repo.ElapsedIndexing)
	return
}

type rootstripper struct {
	advance int
}

func newRootStripper(rootpath string) rootstripper {
	advance := len(rootpath)
	if !strings.HasSuffix(rootpath, string(os.PathSeparator)) {
		advance++
	}
	return rootstripper{advance}
}

func (rs rootstripper) suffix(s string) string {
	return s[rs.advance:]
}

// The scanner returns files eligible for indexing
func (i *indexer) scanner(ctx context.Context, query *IndexQuery) ([]string, error) {
	var err error
	rs := newRootStripper(query.Root)
	names := make([]string, 0)

	// First, add any specific files in the request
	for _, name := range query.Files {
		names = append(names, name)
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
				names = append(names, rs.suffix(p))
			}
			return nil
		}
		path = filepath.Join(query.Root, path)
		err = filepath.Walk(path, walker)
	}
	return names, err
}

func indexShard(
	i *indexer,
	q *IndexQuery,
	writer index.IndexWriter,
	in chan string,
	out chan int) par.RequestFunc {

	// While there are files to add, add them to the specified shard.
	return func(ctx context.Context) error {
		numFiles := 0
		for name := range in {
			if r, err := i.sourcer.Reader(name); err == nil {
				writer.Add(name, r)
				_ = r.Close()
				numFiles++
			}
		}
		out <- numFiles
		return nil
	}

}
