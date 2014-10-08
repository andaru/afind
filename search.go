package afind

import (
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andaru/codesearch/index"
	"github.com/andaru/codesearch/regexp"
)

type searcher struct {
	repos  KeyValueStorer
	client *RpcClient
}

func newSearcher(repos KeyValueStorer) *searcher {
	return &searcher{repos: repos}
}

func newSearcherRemote(repos KeyValueStorer, address string) (*searcher, error) {
	var s *searcher

	client, err := NewRpcClient(address)
	if err == nil {
		s = &searcher{repos: repos, client: client}
	}
	return s, err
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
	log.Info("Search [%v] path: [%v] keys: %v cs: %v",
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
		if isSearchLocal(repo) {
			shards, _ := findShards(repo.IndexPath)
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

	log.Debug("search waiting for %d shards", total)
	timeout := time.After(30 * time.Second)
	for total > 0 {
		select {
		case <-timeout:
			break
		case newSr := <-ch:
			mergeResponse(newSr, sr)
			total--
		}
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

func isSearchLocal(repo *Repo) bool {
	localhost, _ := config.DefaultRepoMeta["hostname"]
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

	if s.client == nil {
		err = newNoRepoAvailableError()
		return newPopSearchResponse(repo, err), err
	}
	request.RepoKeys = []string{repo.Key}
	resp, err = s.client.Search(request)
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
func newGrep(repo *Repo, filename string) *grep {
	return &grep{repo: repo, filename: filename}
}

// Returns a slice of paths found for a given Repo.IndexPath prefix
func findShards(indexPath string) ([]string, error) {
	results, err := filepath.Glob(indexPath + "*.afindex")
	return results, err
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

	// Open the index file
	ix, err := index.Open(s.filename)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return
	}

	// Generate the trigram query from the search regexp
	s.Regexp = re
	q := index.RegexpQuery(re.Syntax)

	// Perform the trigram index search
	var post []uint32
	post = ix.PostingQuery(q)

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

	return
}

func (s *grep) grepFile(filename string) (int, map[string]string, error) {
	fname := path.Join(s.repo.Root, filename)
	f, err := os.Open(fname)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
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
			matches[strconv.Itoa(lineno)] = string(
				buf[lineStart:lineEnd])
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
	return lineno - 1, matches, err
}
