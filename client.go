package afind

// A JSON/HTTP REST client to the afind web service

import (
	"github.com/golang/glog"
	"github.com/jmcvetta/napping"
	"github.com/martini-contrib/binding"
)

const (
	DEFAULT_PORT = `12080`
)

func remoteSearch(source *Source, request SearchRequest, response *SearchResponse) (
	status int, err error) {

	var errs *binding.Errors

	if source.Host == "" {
		panic("source Host must not be empty")
	}

	uri := `http://` + source.Host + `:` + DEFAULT_PORT + `/search`

	s := napping.Session{}
	httpresp, err := s.Post(uri, request, response, errs)
	glog.Info("err=%v httpresp=%+v", err, httpresp)
	return httpresp.Status(), err
}

func restReq(uri string, request SearchRequest, response *SearchResponse) (int, error) {
	s := napping.Session{}
	// TODO errs, not nil
	httpresp, err := s.Post(uri, request, response, nil)
	glog.V(6).Infof("%s err=%v httpresp=%#v", FN(), err, httpresp)
	return httpresp.Status(), err
}

func remoteIndex(source *Source) (
	status int, err error) {

	var errs *binding.Errors

	if source.Host == "" {
		panic("source Host must not be empty")
	}

	uri := `http://` + source.Host + `:` + DEFAULT_PORT + `/search`
	s := napping.Session{}
	httpresp, err := s.Post(uri, source, source, errs)
	glog.Infof("%s err=%v httpresp=%+v", FN(), err, httpresp)
	glog.Infof("%s source=%+v", FN(), source)
	return httpresp.Status(), err
}
