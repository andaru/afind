package afind

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andaru/codesearch/index"
)

type indexer struct {
	svc   Service
	repos KeyValueStorer
}

func newIndexer(svc Service) indexer {
	return indexer{svc: svc, repos: svc.repos}
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
	if i.svc.config.IndexInRepo {
		return path.Join(request.Root, request.Key)
	}
	return path.Join(i.svc.config.IndexRoot, request.Key)
}

func (i indexer) indexLocal(request IndexRequest) (resp *IndexResponse, err error) {
	start := time.Now()
	if err := validateIndexRequest(&request, i.repos); err != nil {
		log.Debug("index %v failed: %v", request.Key, err)
		return nil, err
	}

	resp = newIndexResponse()
	repo := newRepoFromIndexRequest(&request)
	resp.Repo = &repo
	repo.IndexPath = i.indexPathPrefix(&request)

	// Always create at least one shard
	numShards := i.svc.config.NumShards
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
	noireg := i.svc.config.NoIndex()

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
		resp.Error = err.Error()
	} else {
		repo.State = OK
	}

	return
}

func (i indexer) indexRemote(request IndexRequest) (resp *IndexResponse, err error) {
	defaultPort := i.svc.config.DefaultRepoMeta["port.rpc"]
	addr := metaRpcAddress(request.Meta, defaultPort)
	log.Debug("index remote [%s]", addr)

	client, err := i.svc.remotes.Get(addr)
	if client == nil {
		return nil, newNoRpcClientError()
	}
	resp, err = client.Index(request)
	return resp, err
}

func (i indexer) Index(request IndexRequest) (resp *IndexResponse, err error) {
	start := time.Now()
	log.Info("index %v root: %v meta: %v (%d dirs)",
		request.Key, request.Root, request.Meta, len(request.Dirs))
	log.Debug("index %v recursive: %v dirs: %v",
		request.Key, request.Recurse, request.Dirs)

	if request.Meta == nil {
		request.Meta = make(map[string]string)
	}
	// Metedata: merge in empty keys from default repo metadata
	for k, v := range i.svc.config.DefaultRepoMeta {
		if _, ok := request.Meta[k]; !ok {
			request.Meta[k] = v
		}
	}

	// Go local or proxy to a remote afindd
	if i.isIndexLocal(&request) {
		resp, err = i.indexLocal(request)
	} else if request.Recurse {
		request.Recurse = false
		resp, err = i.indexRemote(request)
	}
	if err == nil && resp != nil && resp.Repo != nil {
		if e := i.repos.Set(resp.Repo.Key, resp.Repo); e != nil {
			log.Critical("error setting key %s: %v",
				resp.Repo.Key, e.Error())
		}
	}
	log.Debug("index %v completed in %v", request.Key, time.Since(start))
	if err != nil {
		log.Error(err.Error())
	}
	return
}

func (i indexer) isIndexLocal(request *IndexRequest) (local bool) {
	defaultPort := i.svc.config.DefaultRepoMeta["port.rpc"]
	localaddr := metaRpcAddress(i.svc.config.DefaultRepoMeta, defaultPort)
	addr := metaRpcAddress(request.Meta, defaultPort)
	if localaddr == ":" {
		local = true
	} else if localaddr != addr {
		if strings.HasPrefix(localaddr, request.Meta["host"]) {
			local = true
		}
	} else {
		local = true
	}
	log.Debug("isIndexLocal local=%v other=%v isLocal=%v",
		localaddr, addr, local)
	return local
}
