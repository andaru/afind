package afind

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/andaru/codesearch/index"
	"github.com/andaru/codesearch/regexp"
	"github.com/golang/glog"
)

type searcher struct {
	repos KeyValueStorer
}

func newSearcher(repos KeyValueStorer) *searcher {
	return &searcher{repos}
}

func merge(in *SearchRepoResponse, out *SearchResponse) {
	// todo
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

	start := time.Now()

	repos := make(map[string]*Repo)
	for _, key := range request.RepoKeys {
		r := s.repos.Get(key)
		if r != nil {
			repos[key] = r.(*Repo)
		}
	}
	if len(repos) == 0 {
		// Search all repos
		s.repos.ForEach(func(key string, value interface{}) bool {
			repos[key] = value.(*Repo)
			return true
		})
	}

	// Search specific repos concurrently
	if len(repos) > 0 {
		doneCh := make(chan *SearchRepoResponse)

		count := 0
		for _, repo := range repos {
			count++
			go func() {
				newSr := newSearchRepoResponse()
				newSr, err = s.searchOne(repo, request)
				doneCh <- newSr
			}()
		}
		timeout := time.After(30 * time.Second)
		rcount := 0
		select {
		case newSr := <-doneCh:
			rcount++
			merge(newSr, sr)
		case <-timeout:
			break
		}
		sr.ElapsedNs = time.Since(start).Nanoseconds()
		return sr, err
	}
	return nil, errors.New("no repos")
}

func (s *searcher) searchOne(repo *Repo, request SearchRequest) (
	*SearchRepoResponse, error) {

	g := newGrep(repo)
	reporesp, err := g.searchRepo(&request)
	reporesp.Repo = repo
	return reporesp, err
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

func newGrep(repo *Repo) *grep {
	return &grep{repo: repo}
}

func (s *grep) searchRepo(request *SearchRequest) (
	resp *SearchRepoResponse, err error) {

	fname, _ := normalizeUri(s.repo.UriIndex)
	resp = newSearchRepoResponse()

	pattern := "(?m)" + request.Re
	if !request.CaseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}

	// Attach the regular expression matcher, now setup, to
	// this struct
	s.Regexp = re

	var fileRe *regexp.Regexp
	if request.PathRe != "" {
		fileRe, err = regexp.Compile(request.PathRe)
		if err != nil {
			return
		}
	}

	// try to open the file for reading.
	_, err = os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return
	}

	// Error indication is clear, and can be reset.
	err = nil

	ix := index.Open(fname)
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
		numlines, matches, err := s.grepFile(name)
		resp.Matches[name] = matches
		resp.NumLines = numlines
		if err != nil {
			s.err = err
			glog.Error(name, err)
		}
	}

	return
}

func (s *grep) grepFile(filename string) (int, map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
	return s.reader(f, filename)
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
