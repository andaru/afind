package afind

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"

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

func (self *SourceState) UnmarshalJSON(b []byte) (err error) {
	str := string(b)
	switch str {
	case "INDEXING":
		*self = S_INDEXING
	case "AVAILABLE":
		*self = S_AVAILABLE
	case "ERROR":
		*self = S_ERROR
	default:
		*self = S_NULL
	}
	return nil
}

// An afind.Source represents metadata about an indexed source of files.
//
// A Source is a single shard within the code search system.
type Source struct {
	// The key for this source shard
	Key string `json:"key"`
	// The hostname containing the source (empty is local)
	Host string `json:"host"`
	// The path to prefix all Paths with
	RootPath string `json:"root_path"`
	// The path to the source's index file
	IndexPath string `json:"index_path"`
	// Data to index is in these paths
	Paths []string `json:"paths"`
	// The state of this source's index
	State SourceState `json:"state,string,omitempty"`
	// Source metadata (matched by requests)
	Meta map[string]string `json:"meta"`
	// Number of files indexed
	FilesIndexed int `json:"num_files"`
	// Number of directories in index
	NumDirs int `json:"num_dirs"`
	// Number of nanoseconds spent indexing
	TimeIndexing int64

	// Regexp of files to skip indexing, default if empty
	Noindex string `json:"noindex"`

	// Indexing event
	t *Event
	// Number files skipped
	filesSkipped int
	// The local or remote indexer for this source
	indexer Indexer
}

func NewSource(key string, index string) *Source {
	return &Source{
		Key:       key,
		IndexPath: index,
		Paths:     make([]string, 0)}
}

func NewSourceWithPaths(key string, index string, paths []string) *Source {
	s := &Source{
		Key:       key,
		IndexPath: index,
		Paths:     make([]string, len(paths))}

	for i, path := range paths {
		s.Paths[i] = path
	}
	return s
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

func (s *localIndexer) Index() error {
	var err error
	var reg *regexp.Regexp

	s.src.State = S_INDEXING
	s.src.t.Start()
	defer s.src.t.Stop()
	glog.Infof("Index %+v", s.src)

	if s.src.Noindex != "" {
		reg, err = regexp.Compile(s.src.Noindex)
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
		glog.Infof("err: %s %#v", ixfilename, err)
		return err
	}
	glog.V(6).Infof("creating source index file: %s", ixfilename)
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

func (s *remoteIndexer) Index() error {
	status, err := remoteIndex(s.src)

	glog.V(6).Info(FN(), " status=", status, " err=", err)
	return err
}

func (s *Source) Index() error {
	s.t = NewEvent()
	if s.IsLocal() {
		s.indexer = &localIndexer{s}
	} else {
		master := flag.Lookup("master").Value.String()
		glog.Info("master: ", master)
		if master == "false" {
			return NewApiError("RemoteOperationError",
				"This slave can only perform local requests")
		}
		s.indexer = &remoteIndexer{s}
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
			err = NewApiError(
				`IOError`, `Could not open index `+ixfilename)
		}
	} else {
		err = NewApiError(`RemoteOperationError`,
			`This slave can only perform local requests`)
	}
	return
}
