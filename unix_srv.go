package tinyjail

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

func httpRoute() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, world")
	})

	return mux
}

func ListenAndServe(addr string) error {
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: addr,
		Net:  "unix",
	})

	if err != nil {
		return err
	}

	srv := new(http.Server)
	srv.Handler = httpRoute()
	srv.ReadTimeout = time.Minute
	srv.WriteTimeout = time.Minute
	srv.SetKeepAlivesEnabled(true)

	infoln("http server listen on unix: %s", addr)

	if err := srv.Serve(l); err != nil {
		return err
	}

	debugln("http server exit")

	return nil
}
