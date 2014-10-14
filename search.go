package afind

import (
	"bytes"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/andaru/codesearch/index"
	"github.com/andaru/codesearch/regexp"
)

type searcher struct {
	svc   Service
	repos KeyValueStorer
}

func newSearcher(svc Service) searcher {
	return searcher{svc: svc, repos: svc.repos}
}

func mergeResponse(in *SearchResponse, out *SearchResponse) {
	for file, rmatches := range in.Files {
		if _, ok := out.Files[file]; !ok {
			out.Files[file] = make(map[string]map[string]string)
		}
		for repo, matches := range rmatches {
			out.Files[file][repo] = matches
		}
		// todo: make accurate for overlaps
		out.NumLinesMatched += in.NumLinesMatched
	}
}

func repoMetaMatchesSearch(repo *Repo, request *SearchRequest) bool {
	for k, v := range request.Meta {
		if rv, ok := repo.Meta[k]; ok {
			if v == rv {
				return false
			}
		}
	}
	return true
}

func (s searcher) Search(request SearchRequest) (*SearchResponse, error) {
	log.Info("search [%v] path: [%v] keys: %v cs: %v",
		request.Re, request.PathRe, request.RepoKeys, request.CaseSensitive)

	var err error
	start := time.Now()
	sr := newSearchResponse()

	// Select repos to search based
	repos := make(map[string]*Repo)
	for _, key := range request.RepoKeys {
		if value := s.repos.Get(key); value != nil {
			repos[key] = value.(*Repo)
		}
	}

	if len(repos) == 0 {
		// Consider all repos
		s.repos.ForEach(func(key string, value interface{}) bool {
			repo := value.(*Repo)
			if repoMetaMatchesSearch(repo, &request) {
				repos[key] = repo
			}
			return true
		})
	}

	if len(repos) == 0 {
		return nil, newNoRepoAvailableError()
	}

	// Search repos concurrently
	ch := make(chan *SearchResponse)
	total := 0
	log.Debug("search consulting %d repos", len(repos))

	for _, repo := range repos {
		if s.isSearchLocal(repo) {
			shards := getShards(repo)
			for _, shard := range shards {
				total++
				go func(sh string, r *Repo) {
					newSr, _ := s.searchLocal(r, sh, request)
					ch <- newSr
				}(shard, repo)

			}
		} else {
			total++
			go func() {
				newSr, _ := s.searchRemote(repo, request)
				ch <- newSr
			}()
		}
	}

	timeout := s.svc.config.TimeoutSearch()
	startShardWait := time.Now()
	totalShards := total
	log.Debug("search awaiting %d shards", totalShards)

	for total > 0 {
		select {
		case <-timeout:
			log.Debug("search timed out")
			break
		case newSr := <-ch:
			mergeResponse(newSr, sr)
			total--
		}
	}
	if total == 0 {
		log.Debug("search %d shards returned in %v",
			totalShards, time.Since(startShardWait))
	}
	sr.Elapsed = time.Since(start)
	log.Info("Search [%v] path: [%v] complete in %v (%d repos)",
		request.Re, request.PathRe, sr.Elapsed, len(repos))
	return sr, err
}

func repoHost(repo *Repo) string {
	host, ok := repo.Meta["hostname"]
	if ok {
		return host
	}
	return ""
}

func (s searcher) isSearchLocal(repo *Repo) bool {
	localhost, _ := s.svc.config.DefaultRepoMeta["hostname"]
	if len(localhost) == 0 {
		return true
	}
	host := repoHost(repo)
	if localhost != host {
		return false
	}
	return true
}

func (s *searcher) searchLocal(repo *Repo, fname string, request SearchRequest) (
	resp *SearchResponse, err error) {

	g := newGrep(repo, fname)
	return g.searchRepo(&request)
}

func (s *searcher) searchRemote(repo *Repo, request SearchRequest) (
	resp *SearchResponse, err error) {

	client, err := s.svc.remotes.Get(addressFromRepo(repo))
	if err != nil {
		resp = newPopSearchResponse(repo, err)
	}
	request.RepoKeys = []string{repo.Key}
	resp, err = client.Search(request)
	if resp == nil {
		resp = newPopSearchResponse(repo, err)
	}
	return
}

func newPopSearchResponse(repo *Repo, err error) *SearchResponse {
	sr := newSearchResponse()
	sr.Errors[repo.Key] = err.Error()
	return sr
}

// A grep shadows the codesearch Grep
// object and contains a reference to the Repo
// it is tasked to search.
type grep struct {
	// Emulate a regexp.Grep object
	regexp.Grep
	buf []byte // private from regexp.Grep

	repo     *Repo
	filename string
	err      error
}

// Returns a new local RE2 grepper for this repository
// using the index filename.
func newGrep(repo *Repo, ixfilename string) *grep {
	return &grep{repo: repo, filename: ixfilename}
}

func getShards(repo *Repo) []string {
	res := make([]string, repo.NumShards)
	for i := 0; i < repo.NumShards; i++ {
		res[i] = repo.IndexPath + "-" + strconv.Itoa(i) + ".afindex"
	}
	return res
}

func (s *grep) searchRepo(request *SearchRequest) (
	resp *SearchResponse, err error) {

	// Setup the RE2 expression text based on request options
	pattern := "(?m)" + request.Re
	if !request.CaseSensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}

	var fileRe *regexp.Regexp
	if request.PathRe != "" {
		fileRe, err = regexp.Compile(request.PathRe)
		if err != nil {
			return
		}
	}

	resp = newSearchResponse()

	var ix *index.Index
	// Open the index file
	ix, err = index.Open(s.filename)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return
	}

	pstart := time.Now()
	// Generate the trigram query from the search regexp
	s.Regexp = re
	q := index.RegexpQuery(re.Syntax)

	// Perform the trigram index search
	var post []uint32
	post = ix.PostingQuery(q)
	pelapsed := time.Since(pstart)
	log.Debug("posting query complete in %v", pelapsed)
	// Optionally filter the path names in the posting query results
	if fileRe != nil {
		files := make([]uint32, 0, len(post))
		for _, id_ := range post {
			name := ix.Name(id_)
			if fileRe.MatchString(name, true, true) < 0 {
				continue
			}
			files = append(files, id_)
		}
		post = files
	}

	for _, id_ := range post {
		name := ix.Name(id_)
		numlines, matches, gerr := s.grepFile(name)
		if gerr != nil {
			err = gerr
			log.Error(name, ": ", err)
		}
		resp.Files[name] = make(map[string]map[string]string)
		resp.Files[name][s.repo.Key] = matches
		resp.NumLinesMatched = int64(numlines)
	}
	log.Debug("search shard matched %d lines in %d files",
		resp.NumLinesMatched, len(resp.Files))
	return
}

func (s *grep) grepFile(filename string) (int, map[string]string, error) {
	fname := path.Join(s.repo.Root, filename)
	f, err := os.Open(fname)
	if err != nil {
		return 0, nil, err
	}
	abc := func() {
		if err := f.Close(); err != nil {
			log.Critical(err.Error())
		}
	}
	defer abc() // always. be. closing.

	return s.reader(f, fname)
}

var nl = []byte{'\n'}

func countNL(b []byte) int {
	n := 0
	for {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			break
		}
		n++
		b = b[i+1:]
	}
	return n
}

func (s *grep) reader(r io.Reader, name string) (
	int, map[string]string, error) {

	if s.buf == nil {
		s.buf = make([]byte, 1<<20)
	}

	var (
		err       error
		buf       = s.buf[:0]
		lineno    = 1
		beginText = true
		endText   = false
		matches   = make(map[string]string)
	)

	for {
		n, rerr := io.ReadFull(r, buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		end := len(buf)
		if rerr == nil {
			end = bytes.LastIndex(buf, nl) + 1
		} else {
			endText = true
		}

		chunkStart := 0
		for chunkStart < end {
			m1 := s.Regexp.Match(
				buf[chunkStart:end], beginText, endText) + chunkStart
			beginText = false
			if m1 < chunkStart {
				break
			}
			s.Match = true
			lineStart := bytes.LastIndex(
				buf[chunkStart:m1], nl) + 1 + chunkStart
			lineEnd := m1 + 1
			if lineEnd > end {
				lineEnd = end
			}
			lineno += countNL(buf[chunkStart:lineStart])
			if lineStart != lineEnd {
				matches[strconv.Itoa(lineno)] = string(buf[lineStart:lineEnd])
			}
			lineno++
			chunkStart = lineEnd
		}
		if rerr == nil {
			lineno += countNL(buf[chunkStart:end])
		}
		n = copy(buf, buf[end:])
		buf = buf[:n]
		if len(buf) == 0 && rerr != nil {
			if rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
				err = rerr
			}
			break
		}
	}

	if err != nil {
		return 0, nil, err
	}
	if lineno > 0 {
		lineno--
	}
	return lineno, matches, err
}
