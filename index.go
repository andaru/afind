package afind

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/andaru/codesearch/index"
	"strconv"
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

func (i indexer) indexLocal(request IndexRequest) (resp *IndexResponse, err error) {
	log.Info("local index %v (%d sub dirs)", request.Key, len(request.Dirs))
	start := time.Now()

	if err := validateIndexRequest(&request, i.repos); err != nil {
		log.Debug("local index %v failed: %v", request.Key, err)
		return nil, err
	}

	resp = newIndexResponse()
	repo := newRepoFromIndexRequest(&request)

	numShards := config.NumShards
	if numShards < 1 {
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
	advance := len(request.Root)

	for _, path := range paths {
		lasterr = filepath.Walk(path,
			func(p string, info os.FileInfo, werr error) error {
				// log.Debug("walk path %v info=%+v", path, info)
				if info == nil {
					return nil
				}

				// If a path regular expression was provided,
				// only include files or whole directories
				// that regular expression.
				if reg != nil && reg.FindString(p) != "" {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// Skip excluded extensions and prefixes
				if IndexPathExcludes.MatchFile(p) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				// Track the last walk error if set, then bail
				if werr != nil {
					err = werr
					return nil
				}

				// trim the Repo Root path and trailing slash
				finalpath := p[advance:]

				if info.IsDir() {
					// todo: track only dirs that have files
					numDirs++
				} else if info.Mode()&os.ModeType == 0 {
					// TODO: handle archives

					// add the file to this shard
					slotnum := numFiles % numShards
					shards[slotnum].AddFileInRoot(
						request.Root, finalpath)
					numFiles++
				}
				return nil
			})
	}
	err = lasterr

	// Flush the indices
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
	log.Info("index %s (%d/%d files/dirs) created in %v",
		request.Key, numFiles, numDirs, resp.Elapsed)
	return
}

func (i indexer) indexRemote(request IndexRequest) (resp *IndexResponse, err error) {
	log.Debug("remote index host [%s]", indexRequestHost(&request))
	if i.client == nil {
		return nil, newNoRpcClientError()
	}
	resp, err = i.client.Index(request)
	return resp, err
}

func (i indexer) Index(request IndexRequest) (resp *IndexResponse, err error) {
	// Set meta from defaults then override from request
	for k, v := range config.DefaultRepoMeta {
		request.Meta[k] = v
	}

	if isIndexLocal(&request) {
		resp, err = i.indexLocal(request)
	} else {
		resp, err = i.indexRemote(request)
	}
	if err == nil {
		log.Info("index %v (%s/%s index/data) created in %v",
			request.Key, resp.Repo.SizeIndex, resp.Repo.SizeData, resp.Elapsed)
	}
	return
}

func indexRequestHost(request *IndexRequest) string {
	host, ok := request.Meta["hostname"]
	if ok {
		return host
	}
	return ""
}

func isIndexLocal(request *IndexRequest) bool {
	localhost, _ := config.DefaultRepoMeta["hostname"]
	if len(localhost) == 0 {
		return true
	}
	host := indexRequestHost(request)
	if localhost != host {
		return false
	}
	return true
}
