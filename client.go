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
	glog.V(6).Info(FN(), " backend request to ", uri, " request=", request)

	s := napping.Session{}
	httpresp, err := s.Post(uri, &request, response, errs)
	glog.V(6).Infof("%s done request", FN())
	return httpresp.Status(), err
}

func remoteIndex(source *Source) (
	status int, err error) {

	var errs *binding.Errors

	if source.Host == "" {
		panic("source Host must not be empty")
	}

	uri := `http://` + source.Host + `:` + DEFAULT_PORT + `/sources`
	glog.V(6).Info(FN(), " backend request to ", uri, " source=", source)

	s := napping.Session{}
	httpresp, err := s.Post(uri, source, source, errs)
	if err != nil {
		glog.Infof("http error %s", err)
	}
	glog.V(6).Infof("%s source=%+v", FN(), source)
	return httpresp.Status(), err
}
