package afind

import (
	"time"
	"net/http"

	"github.com/andaru/sub"

	"github.com/go-martini/martini"
	"github.com/golang/glog"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
)

func bindError(field, classification, message string) binding.Error {
	return binding.Error{
		FieldNames: []string{field},
		Classification: classification,
		Message: message}
}

func (s Source) Validate(errs binding.Errors, r *http.Request) binding.Errors {
	if s.Key == "" {
		errs = append(errs, bindError(
			"key", "invalid_input", "must not be empty"))
	}
	if s.RootPath == "" {
		errs = append(errs, bindError(
			"rootpath", "invalid_input", "must not be empty"))
	}
	if s.IndexPath == "" {
		errs = append(errs, bindError(
			"indexpath", "invalid_input", "must not be empty"))
	}
	return errs
}

// Source (Indexing) APIs

func AddSource(src Source, r render.Render, s IterableKeyValueStore) {
	var err error

	glog.V(6).Info("AddSource", src)
	source := NewSourceCopy(src)
	glog.V(6).Info("AddSource", source)
	// Start indexing of uninitialized sources
	if source.State == S_NULL {
		err = source.Index()
	}
	s.Set(source.Key, source)

	if err == nil {
		r.JSON(200, source)
	} else {
		r.Error(500)
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

func UpdateSource(source Source, r render.Render,
	params martini.Params, s IterableKeyValueStore) {

	key := params["key"]
	if _, ok := s.Get(key); ok {
		if source.State == S_NULL {
			err := source.Index()
			if err != nil {
				r.JSON(500, source)
			}
		}
		s.Set(source.Key, source)
		r.JSON(200, source)
	} else {
		r.JSON(404, source)
	}
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
	sources := sourcesForRequest(request, s)

	if len(sources) == 0 {
		r.JSON(200, map[string]string{"error":"no_sources"})
	} else {
		response, err := searchSources(sources, request)
		if err == nil {
			r.JSON(200, response)
		} else {
			r.Error(500)
		}
	}
}

func GetSearch(r render.Render, params martini.Params, s IterableKeyValueStore) {
	request := requestFromParams(params)
	PostSearch(request, r, s)
}

func requestFromParams(params martini.Params) SearchRequest {
	request := NewSearchRequest(params["q"])
	if _, ok := params["cs"]; ok {
		request.Cs = true
	}
	if v, ok := params["path"]; ok {
		request.PathRe = v
	}
	if v, ok := params["key"]; ok {
		request.Key = v
	}
	glog.V(6).Info("requestFromParams", params, request)
	return request
}

func searchSource(source Source, request SearchRequest) (*SearchResponse, error) {
	search := NewSearcherFromSource(source)
	if response, err := search.Search(request); err == nil {
		return response, nil
	} else {
		return nil, err
	}
}

func searchSources(
	sources []*Source, request SearchRequest) (*SearchResponse, error) {

	var err error

	sr := NewSearchResponse()

	// Fire up a background subscription for each source
	srcsubs := make([]sub.Subscriber, 0, len(sources))
	for _, source := range sources {
		srcsubs = append(srcsubs, sub.Subscribe(sub.Source(
			func() sub.SourceResponse {
				r, err := searchSource(*source, request)
				return sub.SourceResponse{Result:r, Err:err}
			})))
	}

	// Run, fan-in and merge the subscriptions, and create a query
	query := sub.Merge(srcsubs...)
	select {
	case merged := <-query.Updates():
		// the merged response
		sr = merged.(*SearchResponse)
		err = query.Close()
	case <-time.After(30 * time.Second):
		// timed out
	}
	return sr, err
}

func sourcesForRequest(request SearchRequest, s IterableKeyValueStore) []*Source {
	sources := make([]*Source, 0)

	if request.Key != "" {
		// Search just a specific shard
		if v, ok := s.Get(request.Key); ok {
			sources = append(sources, v.(*Source))
		}
		// ..else shard not found locally. TODO: ask the master
	} else {
		// Search all shards
		s.ForEach(func(k string, v interface{}) bool {
			// Only include available shards
			shard := v.(*Source)
			if shard.State == S_AVAILABLE {
				sources = append(sources, shard)
			}
			return true  // true continues iteration
		})
	}
	glog.V(6).Infof("search request will use %d/%d sources",
		len(sources), s.Size())
	return sources
}













