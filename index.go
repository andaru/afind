package afind

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/andaru/codesearch/index"
)

const (
	TimeoutIndex = 5 * time.Minute
)

type indexer struct {
	repos   KeyValueStorer
	nshards int
}

func newIndexer(repos KeyValueStorer) *indexer {
	return &indexer{repos, config.NumShards}
}

func tryCreate(fname string) (*os.File, error) {
	oflag := os.O_RDWR | os.O_CREATE | os.O_EXCL
	f, err := os.OpenFile(fname, oflag, 0666)
	if err != nil && os.IsExist(err) {
		// try to remove it and reopen
		err = os.Remove(fname)
		if err != nil {
			return nil, err
		}
		f, err = os.OpenFile(fname, oflag, 0666)
	}
	return f, err
}

func mergeIndexResponse(in *IndexResponse, out *IndexResponse) {
	for key, repo := range in.Repos {
		if repo != nil {
			out.Repos[key] = repo
		}
	}
}

func reposForPrefix(key string, repos KeyValueStorer) []*Repo {
	results := make([]*Repo, 0)
	repos.ForEachSuffix(key, func(k string, v interface{}) bool {
		if r, ok := v.(*Repo); ok {
			results = append(results, r)
			return true
		} else {
			panic(fmt.Sprintf("want *Repo, got %#v", r))
		}
	})
	return results
}

func repoIndexable(key string, repos KeyValueStorer) (indexable bool) {
	indexable = true
	repos.ForEachSuffix(key, func(k string, v interface{}) bool {
		if r, ok := v.(*Repo); ok {
			if r.State < ERROR {
				indexable = false
				return false
			}
			return true
		} else {
			panic(fmt.Sprintf("want *Repo, got %#v", r))
		}
	})
	return
}

// unions for the indexing functions
type irPlusFile struct {
	r *IndexRequest
	f *os.File
}

type respPlusErr struct {
	r *IndexResponse
	e error
}

type indexWriter struct {
	ix []*index.IndexWriter
	repo   []*Repo
}

func getIndexWriter(max int, request *IndexRequest) repoPlusWriter {
	iw := &indexWriter{
		ix: make([]*index.IndexWriter, 0),
		repo: make([]*Repo, 0),
	}
	for int i := 0; i < max; i++ {
		path := getIndexPath(i, request)
		iw.ix[i] = index.Create(path)
	}
	log.Debug("creating index file %s", path)
	writer := index.Create(path)
	shardKey := getShardRequestKey(i, request)
	return repoPlusWriter{writer, newRepo(shardKey, path, request.Meta)}
}

func (i *indexer) Index(request IndexRequest) (resp *IndexResponse, err error) {
	if !repoIndexable(request.Key, i.repos) {
		return nil, NewIndexAvailableError()
	}

	resp = newIndexResponse()
	ixs := make([]repoPlusWriter, config.NumShards)
	for i := 0; i < config.NumShards; i++ {
		ixs[i] = getRepoWriter(i, &request)
	}

	// Buidl the master path list
	paths := make([]string, 0)
	for _, p := range request.Dirs {
		if p != "" && !path.IsAbs(p) {
			paths = append(paths, path.Join(rootPath, p))
		}
	}

	// for _, r := range shardIndexRequest(request) {

	// 	if f, e := tryCreate(r.UriIndex()); e == nil {
	// 		irpf := &irPlusFile{r, f}
	// 		requests = append(requests, irpf)
	// 	} else {
	// 		log.Error("Error creating %s: %v", r.UriIndex(), e)
	// 		return
	// 	}
	// }
	// log.Debug("sharded request into %d", len(requests))
	// Perform indexing concurrently
	ch := make(chan *respPlusErr)
	total := 0

	for _, r := range requests {
		total++
		go func(req *irPlusFile) {
			in, e := makeIndex(*req.r, req.f)
			ch <- &respPlusErr{in, e}
		}(r)
	}
	// Await concurrent results
	for total > 0 {
		select {
		case <-time.After(TimeoutIndex):
			break
		case in := <-ch:
			if in.e != nil {
				err = in.e
			}
			mergeIndexResponse(in.r, resp)
			total--
		}
	}
	// Update the database with the repos indexed
	for key, repo := range resp.Repos {
		err = i.repos.Set(key, repo)
	}
	return
}

func shardIndexRequest(request IndexRequest) []*IndexRequest {
	shards := getShards(request.Dirs, config.NumShards)
	requestShards := make([]*IndexRequest, len(shards))
	for i := range shards {
		req := newIndexRequest(
			fmt.Sprintf("%s_%03d", request.Key, i), request.Root, shards[i])
		requestShards[i] = &req
	}
	return requestShards
}

type repoResp struct {
	sizeData  int64
	sizeIndex int
	numFiles  int
	numDirs   int
}

func makeIndex(request IndexRequest, outf *os.File) (resp *IndexResponse, err error) {
	log.Debug("makeIndex indexFile=%s request=%#v", outf.Name(), request)

	if request.Key == "" {
		return nil, errors.New("request Key must be supplied")
	}

	reg := config.NoIndex()
	start := time.Now()
	repo := newRepoFromIndexRequest(request)
	resp = newIndexResponse()
	rootPath, _ := normalizeUri(request.Root)

	paths := make([]string, 0)
	for _, p := range request.Dirs {
		if p != "" && !path.IsAbs(p) {
			paths = append(paths, path.Join(rootPath, p))
		}
	}

	// Create the index, add paths and then files by walking those paths
	fname := outf.Name()
	log.Info("creating index [%v] at %v %v", repo.Key, fname, paths)
	ix := index.Create(fname)
	ix.AddPaths(paths)

	files, dirs, err := pathwalk(request, paths, reg, ix)
	ix.Flush()
	// todo: re-add merge, and add panic recovery from index.Merge panic

	// Set stats
	repo.Elapsed = time.Since(start).Seconds()
	repo.UriIndex = fname
	repo.NumDirs = dirs
	repo.NumFiles = files
	repo.SizeData = ix.DataBytes()
	repo.SizeIndex = ix.IndexBytes()
	if err != nil {
		// todo: report the error message(s)
		repo.State = ERROR
	}
	resp.Repos[repo.Key] = repo
	log.Info("created index [%v] %v (%d/%d bytes in %.4fs)",
		repo.Key, repo.UriIndex, repo.SizeIndex, repo.SizeData, repo.Elapsed)
	return resp, err
}

const (
	maxShards = 8
)

func min(x, y int) int {
	if x <= y {
		return x
	}
	return y
}

func pathwalk(request IndexRequest,
	numShards int, paths []string, reg *regexp.Regexp,
	ixs *[]*index.IndexWriter) (files, dirs int, err error) {

	var lasterr error

	for _, path := range paths {
		lasterr = filepath.Walk(path,
			func(path string, info os.FileInfo, werr error) error {
				if info == nil {
					return nil
				}
				// skip excluded files, directories
				if reg != nil && reg.FindString(path) != "" {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				// Maybe set the last error
				if werr != nil {
					err = werr
					return nil
				}
				if info.IsDir() {
					dirs++
				}
				if info.Mode()&os.ModeType == 0 {
					// TODO: handle archives
					slotnum := count % numShards
					ix.AddFile(path)
					files++
					// TODO: count errors
				}
				return nil
			})
		err = lasterr
	}
	return
}
