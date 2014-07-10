package afind

// A modification of martini's logger which emits better output

import (
	"log"
	"net/http"
	"time"

	"github.com/go-martini/martini"
)

// Logger returns a middleware handler that logs the request as it
// goes in and the response as it goes out using glog.
func Logger() martini.Handler {
	return func(res http.ResponseWriter, req *http.Request,
		c martini.Context, log *log.Logger) {

			start := time.Now()

			addr := req.Header.Get("X-Real-IP")
			if addr == "" {
				addr = req.Header.Get("X-Forwarded-For")
				if addr == "" {
					addr = req.RemoteAddr
				}
			}

			log.Printf("HTTP_START [%s] %s %s",
				addr, req.Method, req.URL)

			rw := res.(martini.ResponseWriter)
			c.Next()

			log.Printf("HTTP_END [%s] %s %s %v %v",
				addr, req.Method, req.URL, rw.Status(),
				time.Since(start))
		}
}
















