package afind

import (
	"bytes"
	"io"
	"os"
	"strconv"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/stopwatch"
	"github.com/andaru/codesearch/index"
	"github.com/andaru/codesearch/regexp"
	"golang.org/x/tools/godoc/vfs"
)

// grep shadows codesearch.regexp.Grep
type grep struct {
	// Emulate a regexp.Grep object
	regexp.Grep

	// private from regexp.Grep
	buf []byte

	filename string
	root     string
	err      error

	// local private data
	ctxPre  int
	ctxPost int
	ctxBoth int

	fs vfs.FileSystem
}

// Returns a new local RE2 grepper for this repository
// using the index filename and index root path prefix
// (stripped from all filenames added to the index).
func newGrep(ixfilename, root string, fs vfs.FileSystem) *grep {
	return &grep{filename: ixfilename, root: root, fs: fs}
}

// builds regular expressions for text and pathname matching
func buildRegexps(query *SearchQuery) (re, pathre *regexp.Regexp, err error) {
	pattern := "(?m)" + query.Re
	if query.IgnoreCase {
		pattern = "(?i)" + pattern
	}
	re, err = regexp.Compile(pattern)
	if query.PathRe != "" {
		pathre, err = regexp.Compile(query.PathRe)
	}
	return
}

func (s *grep) search(ctx context.Context, query SearchQuery) (
	resp *SearchResult, err error) {

	sw := stopwatch.New()
	resp = NewSearchResult()
	key := query.firstKey()
	var post []uint32
	var ix *index.Index
	var q *index.Query

	// Setup the RE2 expression text based on query options
	re, pathre, err := buildRegexps(&query)
	if err != nil {
		goto done
	}
	s.Regexp = re

	// Attempt to open the index file
	if ix, err = index.Open(s.filename); err != nil {
		log.Debug("grep error opening index %v: %v", s.filename, err)
		goto done
	}

	// Perform the posting query to get candidate files to grep
	sw.Start("posting")
	q = index.RegexpQuery(re.Syntax)
	post = ix.PostingQuery(q)
	// Optionally filter the path names in the posting query results
	if pathre != nil {
		files := make([]uint32, 0, len(post))
		for _, id_ := range post {
			name := ix.Name(id_)
			if pathre.MatchString(name, true, true) < 0 {
				continue
			}
			files = append(files, id_)
		}
		post = files
	}
	resp.Durations.PostingQuery = sw.Stop("posting")

	// Setup context parameters
	s.ctxPre = query.Context.Pre
	s.ctxPost = query.Context.Post
	s.ctxBoth = query.Context.Both

	// Now grep each candidate file to get the final matches
	for _, id_ := range post {
		// check to see if the context has expired each time through
		select {
		case <-ctx.Done():
			err = errs.NewTimeoutError("search")
			goto done
		default:
		}

		name := ix.Name(id_)
		if n, matches, e := s.readfile(name); n == 0 {
			continue
		} else if e != nil && !os.IsNotExist(e) && !os.IsPermission(e) {
			err = e
		} else {
			resp.Matches[name] = make(map[string]map[string]string)
			resp.Matches[name][key] = matches
			resp.NumMatches += uint64(n)
		}
	}

done:
	return
}

func (s *grep) readfile(name string) (int, map[string]string, error) {
	f, err := s.fs.Open(name)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return s.reader(f, name)
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
			m1 := s.Regexp.Match(buf[chunkStart:end], beginText, endText) + chunkStart
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
				// We had a real match, record it
				matches[strconv.Itoa(lineno)] = string(buf[lineStart:lineEnd])
				nmatches++
			} else {
				s.Match = false
			}

			// Set context bounds; if both pre/post and both are set, pick the biggest
			npre := s.ctxBoth
			if s.ctxPre > npre {
				npre = s.ctxPre
			}
			npost := s.ctxBoth
			if s.ctxPost > npost {
				npost = s.ctxPost
			}

			if npre > 0 && s.Match {
				prectx := getPreCtx(buf, 3, chunkStart, lineStart)
				if len(prectx) > 0 {
					ctxline := lineno
					for _, b := range prectx {
						ctxline--
						matches[strconv.Itoa(ctxline)] = string(b)
					}
				}
			}

			if npost > 0 && s.Match {
				postctx := getPostCtx(buf, 3, m1, end)
				if len(postctx) > 0 {
					ctxline := lineno
					for _, b := range postctx {
						ctxline++
						matches[strconv.Itoa(ctxline)] = string(b)
					}
				}
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

// helper function to get the proceeding context

func getPreCtx(b []byte, n, chunkstart, linestart int) [][]byte {
	// todo: don't read from chunkstart every time, that may be
	// a very large amount of data. instead, read backwards until we
	// get N newlines or reach chunkstart.

	result := make([][]byte, 0)
	ctxstart := linestart
	for n > 0 {
		if ctxstart <= chunkstart || len(b) < (ctxstart-chunkstart) {
			// no more data to read
			break
		}
		newctxstart := bytes.LastIndex(b[chunkstart:ctxstart-1], nl) + 1 + chunkstart
		newb := b[newctxstart:ctxstart]
		if len(newb) > 0 {
			result = append(result, newb)
		}
		ctxstart = newctxstart
		n--
	}
	return result
}

func getPostCtx(b []byte, n, chunkstart, end int) [][]byte {
	result := make([][]byte, 0)
	// shortcut for zero lines of context
	if n == 0 {
		return result
	}
	ctxstart := chunkstart
	for n >= 0 {
		if ctxstart >= end {
			break
		}
		newctxstart := bytes.Index(b[chunkstart:end], nl) + 1 + ctxstart
		if newctxstart > end {
			newctxstart = end
		}
		result = append(result, b[ctxstart:newctxstart])
		ctxstart = newctxstart
		n--
	}
	// Strip the first line, which includes the match
	if len(result) > 0 {
		result = result[1:]
	}
	return result
}
