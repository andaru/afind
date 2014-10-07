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

func merge(in *SearchRepoResponse, out *SearchResponse) {
	for file, _ := range in.Matches {
		if _, ok := out.Files[file]; !ok {
			out.Files[file] = make(map[string]map[string]string)
		}
		if _, ok := out.Files[file][in.Repo.Key]; !ok {
			out.Files[file][in.Repo.Key] = make(map[string]string)
		}
		for line, text := range in.Matches[file] {
			out.Files[file][in.Repo.Key][line] = text
		}
		out.NumLinesMatched++
	}
}

func (s *searcher) Search(request SearchRequest) (*SearchResponse, error) {
	var err error
	sr := newSearchResponse()
	log.Info("Search [%v] path: [%v] keys: %v cs: %v",
		request.Re, request.PathRe, request.RepoKeys, request.CaseSensitive)

	start := time.Now()

	// Select repos to search based
	repos := make(map[string]*Repo)
	for _, key := range request.RepoKeys {
		if value := s.repos.Get(key); value != nil {
			repos[key] = value.(*Repo)
		}
	}

	if len(repos) == 0 {
		// Search all repos
		s.repos.ForEach(func(key string, value interface{}) bool {
			repos[key] = value.(*Repo)
			return true
		})
	}

	if len(repos) == 0 {
		return nil, newNoRepoAvailableError()
	}

	// Search specific repos concurrently
	ch := make(chan *SearchRepoResponse)
	total := 0
	for _, repo := range repos {
		shards, err := findShards(repo.IndexPath)
		if err != nil {
		}
		for _, shard := range shards {
			total++
			go func(sh string, r *Repo) {
				newSr := newSearchRepoResponse()
				if !isRemote(r) {
					newSr, err = s.searchOneLocal(sh, r, request)
				} else {
					newSr, err = s.searchOneRemote(r, request)
				}
				ch <- newSr
			}(shard, repo)
		}
	}

	timeout := time.After(30 * time.Second)
	for total > 0 {
		select {
		case <-timeout:
			break
		case newSr := <-ch:
			merge(newSr, sr)
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
	if !ok {
		return ""
	}
	return host
}

func isRemote(repo *Repo) bool {
	ourhost, _ := config.DefaultRepoMeta["hostname"]
	// len() works with both nil and emptry string
	if len(ourhost) == 0 {
		return false
	}

	theirhost := repoHost(repo)
	if theirhost == "" {
		return false
	} else if ourhost != theirhost {
		return true
	}
	return false
}

func (s *searcher) searchOneLocal(fname string, repo *Repo, request SearchRequest) (
	reporesp *SearchRepoResponse, err error) {
	g := newGrep(repo)
	log.Debug("local search [%s]", fname)
	reporesp, err = g.searchRepo(fname, &request)
	reporesp.Repo = repo
	return
}

func (s *searcher) searchOneRemote(repo *Repo, request SearchRequest) (
	reporesp *SearchRepoResponse, err error) {
	if s.client == nil {
		return nil, newNoRpcClientError()
	}
	log.Debug("remote search host [%s]", repoHost(repo))
	reporesp, err = s.client.SearchRepo(repo.Key, request)
	reporesp.Repo = repo
	return
}

// A grep shadows the codesearch Grep
// object and contains a reference to the Repo
// it is tasked to search.
type grep struct {
	// Emulate a regexp.Grep object
	regexp.Grep
	buf []byte // private from regexp.Grep

	repo *Repo
	err  error
}

// Returns a new local RE2 grepper for this repository
func newGrep(repo *Repo) *grep {
	return &grep{repo: repo}
}

func findShards(prefix string) ([]string, error) {
	results, err := filepath.Glob(prefix + "*.afindex")
	return results, err
}

func (s *grep) searchRepo(fname string, request *SearchRequest) (
	resp *SearchRepoResponse, err error) {

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

	resp = newSearchRepoResponse()

	// Open the index file
	ix, err := index.Open(fname)
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
		resp.Matches[name] = matches
		resp.NumLines = numlines
		if gerr != nil {
			err = gerr
			log.Error("%s: %v", name, err)
		}
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
			chunkStart = lineEnd
			lineno++
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
	return lineno, matches, err
}
