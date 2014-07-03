package afind

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"time"

	"code.google.com/p/codesearch/index"
	"code.google.com/p/codesearch/regexp"
	"github.com/golang/glog"
)

type SearchRequest struct {
	Re     string `json:"text"`
	PathRe string `json:"path"`
	Cs     bool `json:"cs"`

	Key string `json:"key"` // The source shard key
}

func NewSearchRequest(textRe string) SearchRequest {
	return SearchRequest{Re: textRe, Cs: false}
}

func NewSearchRequestWithPath(textRe, pathRe string) SearchRequest {
	return SearchRequest{Re: textRe, PathRe: pathRe, Cs: false}
}

type SearchResponse struct {
	// Matches from the query.
	// Keys of the outer map are matching filenames.
	// Keys of the inner map are line numbers in each matching file.
	// Values of the inner map is the matching line.
	M map[string]map[string]string `json:"matches"`

	numMatches int `json:"num_matches"` // the total number of lines matched
}

func (s *SearchResponse) Merge(src *SearchResponse) {
	glog.V(6).Info("merging", s.numMatches, "matches into master response")
	for name, matches := range src.M {
		// TODO: remap the filename based on local rules.
		// For example, we may want to change
		// /var/build/$project/src/Foo to /src/$project/Foo,
		// can use a regex rewrite rule.

		// Insert the new file entry.
		if _, ok := s.M[name]; !ok {
			s.M[name] = make(map[string]string)
		}
		for k, v := range matches {
			// TODO: deal with collisions, which can happen with
			// branching, et. al (if we don't use a rewrite rule
			// to build something of the Perforce path in here).

			// Can use collision to show the diffs between
			// branches either by calculating the diffs in
			// a custom form of this merge function, or by
			// storing all versions of the same line by
			// suffixing "_$key" onto the line number,
			// e.g., "245_17903" for line 245 in project 17903.
			s.M[name][k] = v
		}
	}
}

func NewSearchResponse() *SearchResponse {
	return &SearchResponse{M: make(map[string]map[string]string)}
}

type Searcher interface {
	Search(request SearchRequest) (*SearchResponse, error)
}

type searcher struct {
	// Emulate a regexp.Grep object
	regexp.Grep
	buf []byte // private from regexp.Grep

	indexPath string
	lastErr   error
	t         EventTimer
}

func (s searcher) Elapsed() time.Duration {
	return s.t.Elapsed()
}

func NewSearcher(index string) *searcher {
	return &searcher{t: NewEvent(), indexPath: index}
}

func NewSearcherFromSource(source Source) *searcher {
	return &searcher{t: NewEvent(), indexPath: source.IndexPath}
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

func (s *searcher) Search(request SearchRequest) (
	response *SearchResponse, err error) {

	response = NewSearchResponse()

	s.t.Start()
	defer s.t.Stop()

	if _, err := os.Stat(s.indexPath); os.IsNotExist(err) {
		return nil, err
	}

	pattern := "(?m)" + request.Re
	if !request.Cs {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	s.Regexp = re

	var fileRe *regexp.Regexp
	if request.PathRe != "" {
		fileRe, err = regexp.Compile(request.PathRe)
		if err != nil {
			return nil, err
		}
	}

	// open the index file
	ix := index.Open(s.indexPath)
	q := index.RegexpQuery(re.Syntax)
	var post []uint32
	post = ix.PostingQuery(q)

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
		matches, err := s.GrepFile(name)
		if err != nil {
			glog.Error(name, err)
		}
		if len(matches) > 0 {
			err = nil
			response.M[name] = matches
			response.numMatches += len(matches)
		}
	}
	return
}

func (s *searcher) Reader(r io.Reader, name string) (map[string]string, error) {
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
			line := buf[lineStart:lineEnd]
			matches[strconv.Itoa(lineno)] = string(line)
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
		return nil, err
	}
	return matches, err
}

func (s *searcher) GrepFile(filename string) (m map[string]string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return m, err
	}
	defer f.Close()
	return s.Reader(f, filename)
}
