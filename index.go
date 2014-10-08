package afind

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/andaru/codesearch/index"
	"strconv"
	"strings"
)

const (
	TimeoutIndex = 5 * time.Minute
)

type indexer struct {
	repos   KeyValueStorer
	nshards int
	client  *RpcClient
}

func newIndexer(repos KeyValueStorer) *indexer {
	return &indexer{repos: repos, nshards: config.NumShards}
}

func newIndexerRemote(repos KeyValueStorer, address string) (*indexer, error) {
	var i *indexer

	client, err := NewRpcClient(address)
	if err == nil {
		i = &indexer{repos: repos, nshards: config.NumShards, client: client}
	}
	return i, err
}

func validateIndexRequest(request *IndexRequest, repos KeyValueStorer) error {
	var err error

	if len(request.Dirs) == 0 {
		err = newValueError(
			"Dirs",
			"Requires at least one sub dir of Root, such as '.'")
	} else if !path.IsAbs(request.Root) {
		err = newValueError(
			"Root", "Root must be an absolute path")
	} else if repos.Get(request.Key) != nil {
		err = newIndexAvailableError(request.Key)
	}
	return err
}

func (i *indexer) indexLocal(request IndexRequest) (resp *IndexResponse, err error) {
	log.Info("local index %v (%d sub dirs)", request.Key, len(request.Dirs))
	start := time.Now()

	if err := validateIndexRequest(&request, i.repos); err != nil {
		log.Debug("local index %v failed: %v", request.Key, err)
		return nil, err
	}

	resp = newIndexResponse()
	numShards := config.NumShards
	if numShards == 0 {
		numShards = 1
	}

	shards := make([]*index.IndexWriter, numShards)
	for n := range shards {
		shards[n] = index.Create(path.Join(
			request.Root, request.Key+"-"+strconv.Itoa(n)+".afindex"))
	}

	// Build the master path list from request.Dirs, ignoring absolute paths
	paths := make([]string, 0)
	for _, p := range request.Dirs {
		if p == "" {
			continue
		}
		if !path.IsAbs(p) {
			paths = append(paths, path.Join(request.Root, p))
		} else {
			log.Debug("skip non absolute path '%s'")
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
				// log.Debug("walk path %v info=%+v", path, info)
				if info == nil {
					return nil
				}

				// If a path regular expression was provided,
				// only include files or whole directories
				// that regular expression.
				if reg != nil && reg.FindString(path) != "" {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				// Skip excluded extensions and prefixes
				if IndexPathExcludes.MatchFile(path) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				// Set the last error if we had an IO error walking
				if werr != nil {
					err = werr
					return nil
				}
				if info.IsDir() {
					numDirs++
				} else if info.Mode()&os.ModeType == 0 {
					// TODO: handle archives
					slotnum := numFiles % numShards
					finalpath := strings.TrimPrefix(path, request.Root)[1:]
					shards[slotnum].AddFileInRoot(request.Root, finalpath)
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
		repo.SizeData += ByteSize(ix.DataBytes())
		repo.SizeIndex += ByteSize(ix.IndexBytes())
	}
	if err != nil {
		repo.State = ERROR
	} else {
		repo.State = OK
	}
	repo.IndexPath = path.Join(request.Root, request.Key)
	repo.NumFiles = numFiles
	repo.NumDirs = numDirs
	// resp.Repos[repo.Key] = repo
	for k, v := range request.Meta {
		repo.Meta[k] = v
	}
	resp.Repo = repo
	resp.Elapsed = time.Since(start)
	err = i.repos.Set(repo.Key, repo)
	log.Info("local index %v (%d files/%d dirs) finished in %v",
		request.Key, numFiles, numDirs, resp.Elapsed)
	log.Debug("repo=%#v", resp.Repo)
	return
}

func (i *indexer) indexRemote(request IndexRequest) (resp *IndexResponse, err error) {
	log.Debug("remote index host [%s]", indexRequestHost(&request))
	if i.client == nil {
		return nil, newNoRpcClientError()
	}
	resp, err = i.client.Index(request)
	return resp, err
}

func (i *indexer) Index(request IndexRequest) (resp *IndexResponse, err error) {
	// Set meta from defaults then override from request
	for k, v := range config.DefaultRepoMeta {
		request.Meta[k] = v
	}

	if isIndexRequestLocal(&request) {
		return i.indexLocal(request)
	} else {
		return i.indexRemote(request)
	}
}

func indexRequestHost(request *IndexRequest) string {
	host, ok := request.Meta["hostname"]
	if !ok {
		return ""
	} else {
		return host
	}
}

func isIndexRequestLocal(request *IndexRequest) bool {
	host := indexRequestHost(request)
	localhost, _ := config.DefaultRepoMeta["hostname"]
	if len(localhost) == 0 {
		return true
	}
	if localhost != host {
		return false
	}
	return true
}
