package afind

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"code.google.com/p/codesearch/index"
	"github.com/golang/glog"
)

const (
	DEFAULT_NOINDEX_REGEXP = `"\(\.\(rpm$|gz$|bz2$|zip$\)` +
		`\|/autogen/\|autom4te\.ca\|RPMS/cachedir\)"`
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
	Key       string      `json:"key"`          // The key for this source shard
	Host      string      `json:"host"`         // The hostname containing the source (empty is local)
	RootPath  string      `json:"rootpath"`     // The path to prefix all Paths with
	IndexPath string      `json:"indexpath"`    // The path to the source's index file
	Paths     []string    `json:"paths"`        // Data to index is in these paths
	State     SourceState `json:"state,string"` // The state of this source's index

	filesIndexed int
	filesSkipped int
	numDirs      int
	noindex      string
	t            *Event
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

func NewSourceCopy(src Source) *Source {
	return &Source{
		Key:       src.Key,
		IndexPath: src.IndexPath,
		RootPath:  src.RootPath,
		Paths:     src.Paths,
		t:         NewEvent()}
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

func (s *Source) Index() error {
	var err error
	var reg *regexp.Regexp

	s.t.Start()
	defer s.t.Stop()

	noindex := s.noindex
	if noindex == "" {
		noindex = DEFAULT_NOINDEX_REGEXP
	}
	glog.V(6).Infof("indexing %d paths for key [%s]", len(s.Paths), s.Key)

	reg, err = regexp.Compile(noindex)
	if err != nil {
		return err
	}

	// Remove any bogus and empty paths
	for i, p := range s.Paths {
		p = path.Join(s.RootPath, p)
		if absPath, err := filepath.Abs(p); err == nil {
			s.Paths[i] = absPath
		} else {
			glog.Errorf("error %s: %s", p, err)
			// Delete this entry rather than setting it
			s.Paths = append(s.Paths[:i], s.Paths[i+1:]...)
		}
	}

	// Start indexing - this may crash here if s.IndexPath doesn't exist
	ix := index.Create(s.IndexPath)
	ix.AddPaths(s.Paths)

	for _, path := range s.Paths {
		filepath.Walk(path,
			func(path string, info os.FileInfo, werr error) error {
				if info == nil {
					return nil
				}
				// skip excluded files, directories
				if noindex != "" && reg.FindString(path) != "" {
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
					s.numDirs++
				}
				// Maybe set the last error
				if werr != nil {
					err = werr
					return nil
				}
				if info != nil && info.Mode()&os.ModeType == 0 {
					ix.AddFile(path)
					// TODO: count errors
					s.filesIndexed++
				}
				return nil
			})
	}
	ix.Flush()

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
		s.State = S_ERROR
	} else {
		s.State = S_AVAILABLE
	}
	glog.V(6).Infof("indexed %d total directories\n", s.numDirs)
	return err
}
