package afind

// Provides martini HTTP handlers for the afind web service

import (
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/go-martini/martini"
)

var (
	m *martini.ClassicMartini
)

func apiRouter() martini.Router {
	r := martini.NewRouter()
	return r
}

func AfindServer() *martini.ClassicMartini {
	if m != nil {
		return m
	}
	m = martini.Classic()
	m.Use(render.Renderer())
	// m.Use(Logger())
	// Inject the database interface so
	// it is available to handlers
	database := NewKvstore()
	m.Map(&database)
	// Add API router endpoints
	m.Post(`/sources`, binding.Bind(Source{}), AddSource)
	m.Delete(`/sources/:key`, DeleteSource)

	m.Get(`/sources`, GetSources)
	m.Get(`/sources/:key`, GetSource)
	m.Get(`/sources/:key/paths`, GetSourcePaths)

	m.Get(`/search`, GetSearch)
	m.Post(`/search`, binding.Bind(SearchRequest{}), PostSearch)

	return m
}
