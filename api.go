package afind

import (
	"net/http"
	"time"
	"strings"

	"github.com/andaru/sub"

	"github.com/go-martini/martini"
	"github.com/golang/glog"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

const (
	TIMEOUT_QUERY = 30 * time.Second
)

func bindError(field, classification, message string) binding.Error {
	return binding.Error{
		FieldNames:     []string{field},
		Classification: classification,
		Message:        message}
}

func (s SearchRequest) Validate(errs binding.Errors, r *http.Request) *binding.Errors {
	glog.V(6).Infof(FN(), " %+v", s)
	e := make(binding.Errors, 0)
	e = append(e, errs...)
	if len(s.Re) <= 2 {
		e = append(e, bindError(
			"q", "invalid_input", "must be 3 or more characters"))
	}
	if len(s.Re) <= 5 && (strings.Contains(s.Re, ".*") ||
		strings.Contains(s.Re, ".+")) {

		e = append(e, bindError(
			"q", "too_broad", "regexp is too permissive"))
	}
	return &e
}

func (s Source) Validate(errs binding.Errors, r *http.Request) *binding.Errors {
	glog.V(6).Infof(FN(), " %+v", s)
	e := make(binding.Errors, 0)
	e = append(e, errs...)
	if s.Key == "" {
		e = append(e, bindError(
			"key", "invalid_input", "must not be empty"))
	}
	if s.RootPath == "" {
		e = append(e, bindError(
			"rootpath", "invalid_input", "must not be empty"))
	}
	if s.IndexPath == "" {
		e = append(e, bindError(
			"indexpath", "invalid_input", "must not be empty"))
	}
	return &e
}

// Source (Indexing) APIs

func AddSource(src Source, r render.Render, s IterableKeyValueStore) {
	var err error

	source := Source(src)
	glog.V(2).Infof("%s %+v", FN(), source)

	// Start indexing of uninitialized sources
	v, ok := s.Get(source.Key)
	if ok && source.State == S_NULL && v.(*Source).State == S_AVAILABLE {
		// Don't reindex existing available indices
		glog.V(2).Infof("%s not clobbering source key %s", FN(), source.Key)
	} else {
		if source.State == S_NULL {
			if source.t == nil {
				source.t = NewEvent()
			}
			// Index the source (perhaps via a backend call)
			err = source.Index()
		}
		s.Set(source.Key, &source)
	}

	if err == nil {
		value, _ := s.Get(source.Key)
		r.JSON(200, value)
	} else {
		r.Error(500)
	}
}

func GetSourcePaths(
	r render.Render, params martini.Params, s IterableKeyValueStore) {
	key := params["key"]
	if v, ok := s.Get(key); ok {
		source := v.(*Source)
		glog.Info(FN(), " ", source)
		if files, err := source.Files(); err == nil {
			r.JSON(200, files)
		} else {
			glog.Error(FN(), " error: ", err.Error())
			r.JSON(500, err)
		}
	}
}

func GetSource(r render.Render, params martini.Params, s IterableKeyValueStore) {
	key := params["key"]
	if source, ok := s.Get(key); ok {
		r.JSON(200, source)
	} else {
		r.JSON(404, source)
	}
}

func GetSources(r render.Render, s IterableKeyValueStore) {
	data := getSources(s)
	r.JSON(200, data)
}

func DeleteSource(r render.Render, params martini.Params, s IterableKeyValueStore) {
	key := params["key"]
	if _, ok := s.Get(key); ok {
		s.Delete(key)
		r.JSON(200, key)
	} else {
		r.JSON(404, key)
	}
}

func getSources(s IterableKeyValueStore) map[string]interface{} {
	r := make(map[string]interface{})
	s.ForEach(func(key string, value interface{}) bool {
		r[key] = value
		return true
	})
	return r
}

// Search APIs

func PostSearch(request SearchRequest, r render.Render, s IterableKeyValueStore) {
	response, err := doSearch(request, s)
	if err != nil {
		r.JSON(500, binding.Errors{*err})
	} else {
		r.JSON(200, response)
	}
}

func GetSearch(req *http.Request, r render.Render, s IterableKeyValueStore) {
	sr := NewSearchRequest()
	updateRequestFromParams(req, &sr)
	errs := validateGetSearch(req, &sr)
	if len(*errs) > 0 {
		r.JSON(500, errs)
	} else {
		response, err := doSearch(sr, s)
		if err != nil {
			r.JSON(500, binding.Errors{*err})
		} else {
			r.JSON(200, response)
		}
	}
}

func doSearch(request SearchRequest, s IterableKeyValueStore) (
		response *SearchResponse, err *binding.Error) {

	sources := sourcesForRequest(request, s)
	glog.Infof("Search %+v (%d/%d sources)", request, len(sources), s.Size())
	if len(sources) == 0 {
		return nil, &binding.Error{
			Classification: "search",
			Message: "No source code indices were found for this request"}
	} else {
		// Concurrently search sources
		response, err := searchSources(sources, request)
		if err == nil {
			return response, nil
		} else {
			return nil, &binding.Error{
				Classification: "search",
				Message: err.Error()}
		}
	}
}

// Get the query parameters from a GET request
func updateRequestFromParams(req *http.Request, r *SearchRequest) {
	query := req.URL.Query()
	if q := query.Get("q"); q != "" {
		r.Re = q
	}
	if cs := query.Get("cs"); cs != "" {
		r.Cs = true
	}
	if path := query.Get("path"); path != "" {
		r.PathRe = path
	}
	if key := query.Get("key"); key != "" {
		r.Key = key
	}
	if project := query.Get("project"); project != "" {
		r.Meta["project"] = project
	}
}

// Wrap the binding Validation for GET requests
func validateGetSearch(req *http.Request, sr *SearchRequest) *binding.Errors {
	return sr.Validate(binding.Errors{}, req)
}

func searchSource(source *Source, request SearchRequest) (*SearchResponse, error) {
	var search Searcher
	// If the source is local (i.e., host is us or host is empty), create a
	// local searcher, else use a remote searcher (via HTTP POST).
	if source.IsLocal() {
		search = NewSearcher(*source)  // local search
	} else {
		search = NewRemoteSearcher(*source)
	}
	// Perform the search
	return search.Search(request)
}

type srcr struct {
	source  *Source
	request SearchRequest
}

func (s srcr) Fetch() sub.SourceResponse {
	r, err := searchSource(s.source, s.request)
	return sub.SourceResponse{Result: r, Err: err}
}

// Search all sources for the request concurrently
func searchSources(
	sources []*Source, request SearchRequest) (*SearchResponse, error) {

	var err error

	sr := NewSearchResponse()

	// Create a new background subscription for each source
	srcsubs := make([]sub.Subscriber, 0, len(sources))
	for i := range sources {
		srcsubs = append(srcsubs, sub.Subscribe(srcr{sources[i], request}))
	}

	seen := 0
	fanin := sub.Merge(srcsubs...)

	// Fan the subscriptions into a merger to return a single response
	for {
		// Are we there yet?
		if seen >= len(srcsubs) {
			err = fanin.Close()
			break
		}

		select {
		case update := <-fanin.Updates():
			// Received an update from the fan-in queue
			seen++
			incoming := update.(*SearchResponse)
			sr.merge(incoming)
		case <-time.After(TIMEOUT_QUERY):  // timeout
			err = fanin.Close()
			// Explicitly return from the timeout handler
			return sr, err
		}
	}
	glog.Infof("Search %+v found %d matches in %d files",
		request, sr.NLinesMatched, len(sr.M))
	return sr, err
}

// Returns true if the metadata in the Source shard matches the request
//
// Returns true if the request matches all shard
func metaMatch(shard *Source, request SearchRequest) bool {
	for rk, rv := range request.Meta {
		if v, ok := shard.Meta[rk]; ok {
			if rv != v {
				return false
			}
		}
	}
	glog.V(6).Info(FN(), " shard ", shard.Key, " matches ", request.Meta)
	return true
}

func sourcesForRequest(request SearchRequest, s IterableKeyValueStore) []*Source {
	sources := make([]*Source, 0)

	if request.Key != "" {
		// Search just a specific shard
		if v, ok := s.Get(request.Key); ok {
			sources = append(sources, v.(*Source))
		}
		// ..else shard not found locally. TODO: ask the master
	} else if len(request.Meta) > 0 {
		// Search a subset of shards based on the metadata
		s.ForEach(func(k string, v interface{}) bool {
			shard := v.(*Source)
			if metaMatch(shard, request) {
				sources = append(sources, shard)
			}
			return true // continue iteration
		})
	} else {
		// Search all shards
		s.ForEach(func(k string, v interface{}) bool {
			// Only include available shards
			shard := v.(*Source)
			if shard.State == S_AVAILABLE {
				sources = append(sources, shard)
			}
			return true // true continues iteration
		})
	}
	return sources
}







