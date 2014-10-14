package afind

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/andaru/codesearch/index"
	"strconv"
)

type indexer struct {
	repos  KeyValueStorer
	config Config
	client *rpcClient
}

func newIndexer(repos KeyValueStorer, c Config) *indexer {
	return &indexer{repos: repos, config: c}
}

func newIndexerRemote(repos KeyValueStorer, c Config, address string) (*indexer, error) {
	var i *indexer

	client, err := NewRpcClient(address)
	if err == nil {
		i = &indexer{repos: repos, config: c, client: client}
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

func (i indexer) indexPathPrefix(request *IndexRequest) string {
	if i.config.IndexInRepo {
		return path.Join(request.Root, request.Key)
	}
	return path.Join(i.config.IndexRoot, request.Key)
}

func (i indexer) indexLocal(request IndexRequest) (resp *IndexResponse, err error) {
	log.Info("local index %v (%d sub dirs)", request.Key, len(request.Dirs))
	log.Debug("request %+v", request)
	start := time.Now()

	if err := validateIndexRequest(&request, i.repos); err != nil {
		log.Debug("local index %v failed: %v", request.Key, err)
		return nil, err
	}

	resp = newIndexResponse()
	repo := newRepoFromIndexRequest(&request)
	resp.Repo = &repo
	repo.IndexPath = i.indexPathPrefix(&request)

	// Always create at least one shard
	numShards := i.config.NumShards
	if numShards < 1 {
		numShards = 1
	}

	shards := make([]*index.IndexWriter, numShards)
	for n := range shards {
		shards[n] = index.Create(
			repo.IndexPath + "-" + strconv.Itoa(n) + ".afindex")
	}

	// Build the master path list from request.Dirs, ignoring absolute paths
	paths := make([]string, 0)
	for _, p := range request.Dirs {
		if p != "" && !path.IsAbs(p) {
			paths = append(paths, path.Join(request.Root, p))
		} else {
			log.Debug("skipping empty or non absolute path '%s'")
		}
	}

	if len(paths) == 0 {
		return nil, newValueError(
			"Dirs", "No valid paths found")
	}

	// Walk the paths, adding one to each shard round robin
	var lasterr error
	noireg := i.config.NoIndex()

	numDirs := 0
	numFiles := 0
	advance := len(request.Root)

	for _, path := range paths {
		lasterr = filepath.Walk(path,
			func(p string, info os.FileInfo, werr error) error {
				if info == nil {
					return nil
				}
				// If a path regular expression was provided,
				// only include files or whole directories
				// that regular expression.
				if noireg != nil && noireg.FindString(p) != "" {
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

				if info.IsDir() {
					numDirs++
				} else if info.Mode()&os.ModeType == 0 {
					numFiles++
					// trim the Repo Root path and trailing slash
					finalpath := p[advance+1:]
					// add the file to this shard
					slotnum := numFiles % numShards
					shards[slotnum].AddFileInRoot(
						request.Root, finalpath)
				}
				return nil
			})
	}
	err = lasterr

	// Flush the indices
	for _, ix := range shards {
		ix.Flush()
		repo.sizeData += ByteSize(ix.DataBytes())
		repo.sizeIndex += ByteSize(ix.IndexBytes())
	}
	repo.IndexPath = i.indexPathPrefix(&request)
	repo.NumFiles = numFiles
	repo.NumDirs = numDirs
	repo.NumShards = numShards
	for k, v := range request.Meta {
		repo.Meta[k] = v
	}
	repo.SizeData = float64(repo.sizeData)
	repo.SizeIndex = float64(repo.sizeIndex)
	resp.Repo = &repo
	resp.Elapsed = time.Since(start)
	if err != nil {
		repo.State = ERROR
	} else {
		repo.State = OK
	}

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
	log.Info("index %+v", request)
	// Set initial metadata (e.g. host) from server defaults

	if request.Meta == nil {
		request.Meta = make(map[string]string)
	}

	for k, v := range i.config.DefaultRepoMeta {
		request.Meta[k] = v
	}

	if i.isIndexLocal(&request) {
		resp, err = i.indexLocal(request)
	} else {
		resp, err = i.indexRemote(request)
	}
	if err == nil {
		log.Info("index %v (%s/%s index/data) created in %v",
			resp.Repo.Key, resp.Repo.sizeIndex,
			resp.Repo.sizeData, resp.Elapsed)
	}
	if resp != nil {
		err = i.repos.Set(resp.Repo.Key, resp.Repo)
		if err != nil {
			log.Critical("error setting repo key %s: %v",
				resp.Repo.Key, err.Error())
		}
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

func (i indexer) isIndexLocal(request *IndexRequest) bool {
	localhost, _ := i.config.DefaultRepoMeta["hostname"]
	if len(localhost) == 0 {
		return true
	}
	host := indexRequestHost(request)
	if localhost != host {
		return false
	}
	return true
}
