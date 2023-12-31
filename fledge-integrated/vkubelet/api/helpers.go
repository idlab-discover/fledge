package api

import (
	"io"
	"net/http"

	"fledge/fledge-integrated/log"
	"github.com/cpuguy83/strongerrors/status"
)

type handlerFunc func(http.ResponseWriter, *http.Request) error

func handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := f(w, req)
		if err == nil {
			return
		}

		code, _ := status.HTTPCode(err)
		w.WriteHeader(code)
		io.WriteString(w, err.Error())
		logger := log.G(req.Context()).WithError(err).WithField("httpStatusCode", code)

		if code >= 500 {
			logger.Error("Internal server error on request")
		} else {
			log.Trace(logger, "Error on request")
		}
	}
}
