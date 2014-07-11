package afind

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"code.google.com/p/codesearch/index"
	regexp_ "code.google.com/p/codesearch/regexp"
	"github.com/golang/glog"
)

var (
	DEFAULT_NOINDEX_REGEXP = regexp.MustCompile(
		`"\(\.\(rpm$|gz$|bz2$|zip$\)` +
			`\|\/\.\(git|hg|svn\)\/` +
			`\|\/autogen\/` +
			`\|autom4te\.ca` +
			`\|RPMS\/cachedir\)"`)
)

const (
	S_NULL = iota
	S_INDEXING
	S_AVAILABLE
	S_ERROR
)

type badStateChangeError struct {
	s string
}

func (e badStateChangeError) Error() string {
	return e.s
}

func BadStateChangeError(s string) badStateChangeError {
	return badStateChangeError{s}
}

type Indexer interface {
	Index() error
}

type SourceState int

func (s SourceState) String() string {
	switch s {
	case S_INDEXING:
		return "INDEXING"
	case S_AVAILABLE:
		return "AVAILABLE"
	case S_ERROR:
		return "ERROR"
	default:
		return "NULL"
	}
}

// Converts the state to a string for JSON output
func (self SourceState) MarshalJSON() ([]byte, error) {
	return []byte(`"` + self.String() + `"`), nil
}

// An afind.Source represents metadata about an indexed source of files.
//
// A Source is a single shard within the code search system.
type Source struct {
	Key       string            `json:"key"`          // The key for this source shard
	Host      string            `json:"host"`         // The hostname containing the source (empty is local)
	RootPath  string            `json:"rootpath"`     // The path to prefix all Paths with
	IndexPath string            `json:"indexpath"`    // The path to the source's index file
	Paths     []string          `json:"paths"`        // Data to index is in these paths
	State     SourceState       `json:"state,string"` // The state of this source's index
	Meta      map[string]string `json:"meta"`         // Source metadata (matched by requests)

	FilesIndexed int `json:"num_files"` // Number of files indexed
	filesSkipped int
	NumDirs      int `json:"num_dirs"` // Number of directories in index
	noindex      string

	indexer Indexer // The local or remote indexer for this source
	t       *Event
}

func NewSource(key string, index string) *Source {
	return &Source{
		Key:       key,
		IndexPath: index,
		Paths:     make([]string, 0),
		t:         NewEvent()}
}

func NewSourceWithPaths(key string, index string, paths []string) *Source {
	s := &Source{
		Key:       key,
		IndexPath: index,
		Paths:     make([]string, len(paths)),
		t:         NewEvent()}

	for i, path := range paths {
		s.Paths[i] = path
	}
	return s
}

func InitSource(src *Source) {
	src.t = NewEvent()
	if src.Meta == nil {
		src.Meta = make(map[string]string)
	}
}

func abs(s []string) []string {
	result := make([]string, 0)
	for _, v := range s {
		abs_v, err := filepath.Abs(v)
		if err == nil {
			result = append(result, abs_v)
		}
	}
	return result
}

func (s *Source) Elapsed() time.Duration {
	return s.t.Elapsed()
}

func (s *Source) IsLocal() bool {
	hostname, err := os.Hostname()
	if s.Host == "" || (err == nil && hostname == s.Host) {
		return true
	}
	return false
}

type localIndexer struct {
	src *Source
}

type remoteIndexer struct {
	src *Source
}

func (s localIndexer) Index() error {
	var err error
	var reg *regexp.Regexp

	s.src.State = S_INDEXING

	s.src.t.Start()
	defer s.src.t.Stop()

	glog.Infof("Index %+v", s)

	if s.src.noindex != "" {
		reg, err = regexp.Compile(s.src.noindex)
		if err != nil {
			glog.Errorln("noindex regexp syntax error, using default")
			reg = DEFAULT_NOINDEX_REGEXP
		}
	} else {
		reg = DEFAULT_NOINDEX_REGEXP
	}

	// Remove any bogus and empty paths
	for i, p := range s.src.Paths {
		p = path.Join(s.src.RootPath, p)
		if absPath, err := filepath.Abs(p); err == nil {
			s.src.Paths[i] = absPath
		} else {
			glog.Errorf("error %s: %s", p, err)
			// Delete this entry rather than setting it
			s.src.Paths = append(s.src.Paths[:i], s.src.Paths[i+1:]...)
		}
	}

	ixfilename := path.Join(s.src.RootPath, s.src.IndexPath)
	_, err = os.OpenFile(ixfilename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if os.IsExist(err) {
		err = os.Remove(ixfilename)
	} else if err != nil {
		return err
	}
	glog.V(6).Infof("trying to create index: %s", ixfilename)
	ix := index.Create(ixfilename)
	ix.AddPaths(s.src.Paths)
	ix.Flush()
	s.src.pathwalk(reg, ix)

	// Setup a panic recovery deferral for index.Merge()'s burps
	defer func() {
		if r := recover(); r != nil {
			// was it some nasty error? if so, actually panic
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = fmt.Errorf(r.(string))
		}
	}()
	if err != nil {
		s.src.State = S_ERROR
	} else {
		s.src.State = S_AVAILABLE
	}
	glog.V(6).Infof("indexed %d total directories\n", s.src.NumDirs)
	return err
}

func (s remoteIndexer) Index() error {
	status, err := remoteIndex(s.src)

	glog.V(6).Info(FN(), " status=", status, " err=", err)
	return err
}

func (s *Source) Index() error {
	if s.IsLocal() {
		s.indexer = localIndexer{s}
	} else {
		s.indexer = remoteIndexer{s}
	}
	return s.indexer.Index()
}

func (s *Source) pathwalk(reg *regexp.Regexp, ix *index.IndexWriter) error {
	var err error
	for _, path := range s.Paths {
		filepath.Walk(path,
			func(path string, info os.FileInfo, werr error) error {
				if info == nil {
					return nil
				}
				// skip excluded files, directories
				if reg != nil && reg.FindString(path) != "" {
					s.filesSkipped++
					if glog.V(6) {
						glog.V(6).Infoln("skip", path)
					}
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if info.IsDir() {
					s.NumDirs++
				}
				// Maybe set the last error
				if werr != nil {
					err = werr
					return nil
				}
				if info != nil && info.Mode()&os.ModeType == 0 {
					ix.AddFile(path)
					// TODO: count errors
					s.FilesIndexed++
				}
				return nil
			})
	}
	return err
}

func (s *Source) Files() (files []string, err error) {
	files = make([]string, 0)
	if s.IsLocal() {
		ixfilename := path.Join(s.RootPath, s.IndexPath)

		if _, err = os.Open(ixfilename); !os.IsPermission(err) && !os.IsNotExist(err) {
			ix := index.Open(ixfilename)
			re, _ := regexp_.Compile(".*")
			q := index.RegexpQuery(re.Syntax)
			for _, id_ := range ix.PostingQuery(q) {
				glog.Info(id_)
				files = append(files, ix.Name(id_))
			}
		} else {
			err = errors.New(
				`Source index ` + ixfilename + ` not available`)
		}
	} else {
		err = errors.New("Cannot retrieve paths for a remote index")
	}
	return
}
