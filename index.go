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
	if len(request.Dirs) == 0 {
		return newValueError(
			"dirs",
			"Requires at least one sub dir of Root, such as '.'")
	} else if !path.IsAbs(request.Root) {
		return newValueError(
			"root", "Root must be an absolute path")
	}
	return nil
}

func (i indexer) indexPathPrefix(request *IndexRequest) string {
	if i.svc.config.IndexInRepo {
		return path.Join(request.Root, request.Key)
	}
	return path.Join(i.svc.config.IndexRoot, request.Key)
}

func (i indexer) indexLocal(request IndexRequest) (resp *IndexResponse, err error) {
	resp = newIndexResponse()

	start := time.Now()
	if err := validateIndexRequest(&request, i.repos); err != nil {
		log.Debug("index %v failed: %v", request.Key, err)
		return nil, err
	}
	if r := i.repos.Get(request.Key); r != nil {
		err = newRepoExistsError(request.Key)
		if err != nil {
			es := newErrorService(err)
			resp.Error = *es
		}
		resp.Repo = r.(*Repo)
		return
	}
	repo := newRepoFromIndexRequest(&request)
	resp.Repo = &repo
	repo.IndexPath = i.indexPathPrefix(&request)

	// By default, use 2 shards (in e.g., tests)
	numShards := i.svc.config.NumShards
	if numShards == 0 {
		numShards = 2
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
			"dirs", "No directories available")
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
		es := newErrorService(err)
		resp.Error = *es
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
	log.Info("index %v root: %v meta: %v (%d dirs)",
		request.Key, request.Root, request.Meta, len(request.Dirs))

	if request.Meta == nil {
		request.Meta = make(map[string]string)
	}
	// Metedata: merge in empty keys from default repo metadata
	for k, v := range i.svc.config.DefaultRepoMeta {
		if _, ok := request.Meta[k]; !ok {
			request.Meta[k] = v
		}
	}

	// Wrap indexing in a goroutine to allow for a timeout
	log.Debug("index %v awaiting response", request.Key)
	ch := make(chan *IndexResponse, 1)
	go func(r *IndexRequest) {
		gresp, gerr := i.indexLocalOrRemote(r)
		err = gerr
		ch <- gresp
	}(&request)
	select {
	case resp = <-ch: // and we're done!
	case <-i.svc.config.GetTimeoutIndex():
		log.Error("index %v timed out", request.Key)
	}
	return
}

func (i indexer) indexLocalOrRemote(request *IndexRequest) (
	resp *IndexResponse, err error) {

	resp = newIndexResponse()
	start := time.Now()
	// Go local or proxy to a remote afindd
	if i.isIndexLocal(request) {
		resp, err = i.indexLocal(*request)
	} else if request.Recurse {
		request.Recurse = false
		resp, err = i.indexRemote(*request)
		if IsRepoExistsError(err) && resp != nil {
			// allow a response from the backend
			// to fill our repos cache when the
			// error is because the repo exists on
			// the backend.
			err = nil
		}
	}
	if err == nil && resp.Repo != nil {
		repo := resp.Repo
		log.Debug("repo set %v", repo.Key)
		if e := i.repos.Set(repo.Key, repo); e != nil {
			log.Critical("error setting key %s: %v", repo.Key, e.Error())
		}
	}
	log.Debug("index %v completed in %v", request.Key, time.Since(start))
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
