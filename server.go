package afind

// Provides martini HTTP handlers for the afind web service

import (
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/go-martini/martini"
)

var (
	m *martini.Martini
)

func apiRouter() martini.Router {
	r := martini.NewRouter()
	r.Post(`/sources`, binding.Bind(Source{}), AddSource)
	r.Get(`/sources`, GetSources)
	r.Get(`/sources/:key`, GetSource)
	r.Put(`/sources/:key`, UpdateSource)
	r.Delete(`/sources/:key`, DeleteSource)

	r.Get(`/search`, GetSearch)
	r.Post(`/search`, binding.Bind(SearchRequest{}), PostSearch)
	return r
}

func AfindServer() *martini.Martini {
	m = martini.New()
	database := NewKvstore()

	m.Use(martini.Recovery())
	m.Use(martini.Logger())
	m.Use(render.Renderer())
	// Add API router endpoints
	m.Action(apiRouter().Handle)
	// Inject the database
	m.Map(&database)
	return m
}










