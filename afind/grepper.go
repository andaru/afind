package afind

import (
	"bytes"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/codesearch/index"
	"github.com/andaru/codesearch/regexp"
)

// grep shadows codesearch.regexp.Grep
type grep struct {
	// Emulate a regexp.Grep object
	regexp.Grep
	buf []byte // private from regexp.Grep

	filename string
	root     string
	err      error
}

// Returns a new local RE2 grepper for this repository
// using the index filename and index root path prefix
// (stripped from all filenames added to the index).
func newGrep(ixfilename, root string) *grep {
	return &grep{filename: ixfilename, root: root}
}

func (s *grep) search(ctx context.Context, query SearchQuery) (
	resp *SearchResult, err error) {

	resp = NewSearchResult()
	key := query.firstKey()

	// Setup the RE2 expression text based on query options
	var re *regexp.Regexp
	pattern := "(?m)" + query.Re
	if query.IgnoreCase {
		pattern = "(?i)" + pattern
	}
	if re, err = regexp.Compile(pattern); err != nil {
		return
	}
	var fileRe *regexp.Regexp
	if query.PathRe != "" {
		if fileRe, err = regexp.Compile(query.PathRe); err != nil {
			return
		}
	}

	// Check to see if the repo root is still available. If not,
	// return an error so that the caller can mark the repository
	// unavailable.
	if _, err := os.Stat(s.root); err != nil {
		log.Debug("grepper couldn't stat repo root %s: %v", s.root, err)
		return resp, err
	}

	// Attempt to open the index file
	var ix *index.Index
	if ix, err = index.Open(s.filename); err != nil {
		return
	}

	// Perform the posting query to get candidate files to grep
	var post []uint32
	pstart := time.Now()
	s.Regexp = re
	q := index.RegexpQuery(re.Syntax)
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
	pelapsed := time.Since(pstart)
	resp.ElapsedPost = pelapsed

	// Now grep each candidate file to get the final matches
	for _, id_ := range post {
		// check to see if the context has expired each time through
		select {
		case <-ctx.Done():
			err = errs.NewTimeoutError("search")
			return
		default:
		}

		name := ix.Name(id_)
		if n, matches, e := s.readfile(name); n == 0 {
			continue
		} else if e != nil && !os.IsNotExist(e) && !os.IsPermission(e) {
			err = e
		} else {
			resp.Matches[name] = make(map[string]map[string]string)
			// insert our repo key to optimise upstream merges
			resp.Matches[name][key] = matches
			resp.NumMatches += int64(n)
		}
	}
	return
}

func (s *grep) readfile(name string) (int, map[string]string, error) {
	fname := path.Join(s.root, name)
	f, err := os.Open(fname)
	if err != nil {
		return 0, nil, err
	}
	abc := func() {
		if err := f.Close(); err != nil {
			log.Critical("grep file close error:", err.Error())
		}
	}
	defer abc() // always be closing
	return s.reader(f, fname)
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
		nmatches  = 0
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
				nmatches++
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
	return nmatches, matches, err
}

// helper function to count the number of newlines in a byte slice
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
