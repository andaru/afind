package afind

import (
	"errors"
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

func (i *indexer) Index(request IndexRequest) (*IndexResponse, error) {
	var err error
	var resp *IndexResponse

	repo, _ := i.repos.Get(request.Key).(*Repo)
	if repo != nil && repo.State < ERROR {
		// This existing index cannot be re-indexed
		resp = nil
		err = errors.New("can only reindex errored repos")
	} else {
		resp, err = makeIndex(request)
		// Update the database with the repos indexed
		for key, repo := range resp.Repos {
			i.repos.Set(key, repo)
		}
	}
	return resp, err
}

func makeIndex(request IndexRequest) (resp *IndexResponse, err error) {
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

	paths := make([]string, 0)

	// Attempt to open the index file for creation
	fname, _ := normalizeUri(request.UriIndex())
	if _, err = os.OpenFile(
		fname, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666); err != nil {

		if os.IsExist(err) {
			if err = os.Remove(fname); err != nil {
				repo.State = ERROR
				return
			}
		} else if err != nil {
			repo.State = ERROR
			return
		}
	}

	rootPath, _ := normalizeUri(request.Root)

	for _, p := range request.Dirs {
		if p != "" && !path.IsAbs(p) {
			paths = append(paths, path.Join(rootPath, p))
		}
	}

	// Create the index, add paths and then files by walking those paths
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
	// Post the repo result in the collection, for now
	// we only create one result but this simple struct allows
	// for sharding large repos in this function later.
	resp.Repos[repo.Key] = repo
	glog.Infof("created index [%v] %v (%d bytes in %.4fs)",
		repo.Key, repo.UriIndex, repo.SizeIndex, repo.Elapsed)

	return
}

func pathwalk(
	request IndexRequest, paths []string, reg *regexp.Regexp,
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
					// TODO: count errors
					files++
				}
				return nil
			})
	}
	return
}
