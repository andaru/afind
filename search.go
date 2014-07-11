package afind

import (
	"bytes"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"code.google.com/p/codesearch/index"
	"code.google.com/p/codesearch/regexp"
	"github.com/golang/glog"
)

// Request metadata
// Used to fine tune sources chosen by a search
type reqMeta map[string]string

// A search request provided by either a user or an Afind front-end
type SearchRequest struct {
	Re     string  `json:"q" binding:"required"` // The search regular expression
	PathRe string  `json:"path"`                 // Search only in file paths matching this
	Cs     bool    `json:"cs"`                   // Case sensitive? (non-sensitive by default)
	Key    string  `json:"key"`                  // Search only sources with this Source prefix
	Meta   reqMeta `json:"meta"`                 // Search sources matching this metadata query
}

func NewSearchRequest() SearchRequest {
	return SearchRequest{Cs: false, Meta: make(reqMeta)}
}

func NewSearchRequestWithPath(textRe, pathRe string) SearchRequest {
	return SearchRequest{
		Re:     textRe,
		PathRe: pathRe,
		Cs:     false,
		Meta:   make(reqMeta)}
}

// A match line plus source key slice (will become a 2 element json array)
type matchsrc []string

type SearchResponse struct {
	// Matches from the query.
	// Keys of the outer map are matching filenames.
	// Keys of the inner map are line numbers in each matching file.
	// Inner map values are an array of the matching line and source key
	M map[string]map[string][]*matchsrc `json:"matches"`

	// the total number of lines matched
	NLinesMatched int `json:"num_matches"`

	// set on single source responses, not frontend responses
	sourceKey string
}

func (s *SearchResponse) merge(src *SearchResponse) {
	newp := 0
	nummatch := 0

	for name, matches := range src.M {
		// Apply filter transformations to matching paths
		for _, pf := range pathfilter {
			name = pf.Match.ReplaceAllString(name, string(pf.Replace))
		}

		// Insert the new file entry.
		if _, ok := s.M[name]; !ok {
			newp++
			s.M[name] = make(map[string][]*matchsrc)
		}
		for k, v := range matches {
			if _, ok := s.M[name][k]; !ok {
				s.M[name][k] = make([]*matchsrc, 0)
			}
			s.M[name][k] = append(s.M[name][k], v...)
			nummatch++
		}
	}
	s.NLinesMatched = s.NLinesMatched + nummatch
	if newp > 0 {
		glog.V(6).Infof("source key %s has %d matches in %d files",
			src.sourceKey, nummatch, newp)
	}
}

func NewSearchResponse() *SearchResponse {
	return &SearchResponse{M: make(map[string]map[string][]*matchsrc)}
}

func newSearchResponseFromSearcher(s *searcher) *SearchResponse {
	sr := NewSearchResponse()
	sr.sourceKey = s.source.Key
	return sr
}

type Searcher interface {
	Search(request SearchRequest) (*SearchResponse, error)
}

type searcher struct {
	// Emulate a regexp.Grep object
	regexp.Grep
	buf []byte // private from regexp.Grep

	source  *Source
	lastErr error
	t       EventTimer
}

func (s searcher) Elapsed() time.Duration {
	return s.t.Elapsed()
}

func NewSearcher(source Source) *searcher {
	return &searcher{
		source: &source,
		t:      NewEvent()}
}

func NewSearcherFromIndex(path string) *searcher {
	return NewSearcher(Source{IndexPath: path})
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
	response.sourceKey = s.source.Key

	response = newSearchResponseFromSearcher(s)
	s.t = NewEvent()
	s.t.Start()
	defer s.t.Stop()

	ixfilename := path.Join(s.source.RootPath, s.source.IndexPath)
	glog.V(6).Infof("trying source index: %s", ixfilename)
	if _, err := os.Stat(ixfilename); os.IsNotExist(err) {
		glog.Error("index not found: %s (%s)", ixfilename, err)
		return response, err
	}

	pattern := "(?m)" + request.Re
	if !request.Cs {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}
	s.Regexp = re

	var fileRe *regexp.Regexp
	if request.PathRe != "" {
		fileRe, err = regexp.Compile(request.PathRe)
		if err != nil {
			return
		}
	}

	// open the index file
	ix := index.Open(s.source.IndexPath)
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
			s.lastErr = err
			glog.Error(name, err)
		}
		if len(matches) > 0 {
			err = nil
			response.M[name] = matches
			response.NLinesMatched += len(matches)
		}
	}
	return
}

func (s *searcher) Reader(r io.Reader, name string) (map[string][]*matchsrc, error) {
	if s.buf == nil {
		s.buf = make([]byte, 1<<20)
	}

	var (
		err       error
		buf       = s.buf[:0]
		lineno    = 1
		beginText = true
		endText   = false
		matches   = make(map[string][]*matchsrc)
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
			ln := strconv.Itoa(lineno)
			respline := &matchsrc{string(line), s.source.Key}
			matches[ln] = append(matches[ln], respline)
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

func (s *searcher) GrepFile(filename string) (
	m map[string][]*matchsrc, err error) {

	f, err := os.Open(filename)
	if err != nil {
		return m, err
	}
	defer f.Close()
	return s.Reader(f, filename)
}

// Remote searcher interface; for searching remote afind backends

type remoteSearcher struct {
	source  *Source
	lastErr error
	t       EventTimer
}

func (s *remoteSearcher) Search(request SearchRequest) (sr *SearchResponse, err error) {
	sr = NewSearchResponse()
	sr.sourceKey = s.source.Key
	s.t.Start()
	defer s.t.Stop()

	status, err := remoteSearch(s.source, request, sr)
	glog.V(6).Info(FN(), " status=", status, " err=", err)
	return sr, err
}

func NewRemoteSearcher(source Source) *remoteSearcher {
	if source.Host == "" {
		panic("sourceKey must not be empty")
	}
	return &remoteSearcher{
		source: &source,
		t:      NewEvent()}
}
