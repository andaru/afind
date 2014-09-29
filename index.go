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
	"github.com/golang/glog"
)

type indexer struct {
	repos KeyValueStorer
}

func newIndexer(repos KeyValueStorer) *indexer {
	return &indexer{repos}
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
		out.Repos[key] = repo
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

func repoIndexable(key string, repos KeyValueStorer) (indexable, newRepo bool) {
	indexable = true
	newRepo = true
	repos.ForEachSuffix(key, func(k string, v interface{}) bool {
		if r, ok := v.(*Repo); ok {
			newRepo = false
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

const (
	tmpPath = "/tmp"
)

func getIndexFilename(request *IndexRequest) string {
	fn := request.Key + ".afindex"
	if config.IndexInRepo {
		return path.Join(request.Root, tmpPath, fn)
	}
	return path.Join(tmpPath, fn)
}

func (i *indexer) Index(request IndexRequest) (resp *IndexResponse, err error) {
	indexable, newRepo := repoIndexable(request.Key, i.repos)
	if !indexable || !newRepo {
		return nil, NewIndexAvailableError()
	}

	type irPlusFile struct {
		r *IndexRequest
		f *os.File
	}

	var requests []irPlusFile
	resp = newIndexResponse()

	for _, r := range shardIndexRequest(request) {
		if f, nerr := tryCreate(getIndexFilename(&request)); nerr == nil {
			requests = append(requests, irPlusFile{r, f})
		}
	}

	// Perform indexing
	for _, r := range requests {
		in, nerr := makeIndex(*r.r, r.f)
		if v, ok := in.Repos[r.r.Key]; ok {
			copy(v.Dirs, r.r.Dirs)
		}
		mergeIndexResponse(in, resp)
		if nerr != nil {
			err = nerr
		}
	}

	// Update the database with the repos indexed
	for key, repo := range resp.Repos {
		fmt.Printf("setting key=%v value=%#v\n", key, repo)
		err = i.repos.Set(key, repo)
	}
	return
}

func shardIndexRequest(request IndexRequest) []*IndexRequest {
	shards := getShards(request.Dirs, maxShards)
	requestShards := make([]*IndexRequest, len(shards))
	for i := range shards {
		req := newIndexRequest(
			fmt.Sprintf("%s_%03d", request.Key, i), request.Root, shards[i])
		requestShards[i] = &req
	}
	return requestShards
}

func makeIndex(request IndexRequest, outf *os.File) (resp *IndexResponse, err error) {
	var reg *regexp.Regexp

	if request.Key == "" {
		return nil, errors.New("request Key must be supplied")
	}

	if config.Noindex != "" {
		reg, err = regexp.Compile(config.Noindex)
		if err != nil {
			return nil, err
		}
	}

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
	glog.Infof("creating index [%v] at %v %v", repo.Key, fname, paths)
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
	glog.Infof("created index [%v] %v (%d bytes in %.4fs)",
		repo.Key, repo.UriIndex, repo.SizeIndex, repo.Elapsed)
	return
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

func getShards(paths []string, maxShards int) map[int][]string {
	nshards := min(maxShards, len(paths))
	shards := make(map[int][]string, nshards)
	for k := 0; k < nshards; k++ {
		shards[k] = make([]string, 0)
	}
	for i, path := range paths {
		snum := i % nshards
		shard := shards[snum]
		shard = append(shard, path)
		shards[snum] = shard
	}
	return shards
}

func pathwalk(request IndexRequest, paths []string, reg *regexp.Regexp,
	ix *index.IndexWriter) (files, dirs int, err error) {

	for _, path := range paths {
		filepath.Walk(path,
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
					ix.AddFile(path)
					files++
					// TODO: count errors
				}
				return nil
			})
	}
	return
}
