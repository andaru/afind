package afind

import (
	"os"
	"path"
	"path/filepath"
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

func (i *indexer) Index(request IndexRequest) (resp *IndexResponse, err error) {
	if len(request.Dirs) == 0 {
		return nil, NewValueError(
			"Dirs must contain at least one subdirectory of Root")
	} else if !repoIndexable(request.Key, i.repos) {
		return nil, NewIndexAvailableError()
	}

	resp = newIndexResponse()
	numShards := config.NumShards
	if numShards == 0 {
		numShards = 1
	}

	shards := make([]*index.IndexWriter, numShards)
	for n := range shards {
		shards[n] = index.Create(getIndexPath(n, &request))
	}

	// Build the master path list from request.Dirs, ignoring absolute paths
	paths := make([]string, 0)
	for _, p := range request.Dirs {
		if p != "" && !path.IsAbs(p) {
			paths = append(paths, p)
		}
	}

	// Walk the paths, adding files in those paths to each shard in a
	// round-robin way.
	var lasterr error
	reg := config.NoIndex()

	numDirs := 0
	numFiles := 0

	for _, path := range paths {
		lasterr = filepath.Walk(path,
			func(path string, info os.FileInfo, werr error) error {
				if info == nil {
					return nil
				}

				// skip excluded files, directories
				log.Debug("file %s dir? %v", path, info.IsDir())
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
					numDirs++
				} else if info.Mode()&os.ModeType == 0 {
					// TODO: handle archives
					slotnum := numFiles % numShards
					log.Debug("add path [%v] to shard %d", path, slotnum)
					shards[slotnum].AddFile(path)
					numFiles++
				}
				// TODO: count errors
				return nil
			})
	}
	err = lasterr

	// Flush the indices
	repo := newRepoFromIndexRequest(&request)
	for _, ix := range shards {
		ix.Flush()
		log.Debug("flush data/index %vb/%vb", ix.DataBytes(), ix.IndexBytes())
		repo.SizeData += ix.DataBytes()
		repo.SizeIndex += ix.IndexBytes()
	}
	repo.NumFiles = numFiles
	repo.NumDirs = numDirs
	resp.Repos[repo.Key] = repo
	err = i.repos.Set(repo.Key, repo)
	return
}
