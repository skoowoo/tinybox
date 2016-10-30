package tinyjail

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

func httpRoute(send func(event) chan interface{}) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/hello", func(w http.ResponseWriter, r *http.Request) {
		ev := event{
			action: "hello",
		}
		res := <-send(ev)
		json.NewEncoder(w).Encode(res)
	})

	return mux
}

func ListenAndServe(addr string, c chan event) error {
	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: addr,
		Net:  "unix",
	})

	if err != nil {
		return err
	}

	send := func(e event) chan interface{} {
		e.c = make(chan interface{}, 1)
		select {
		case c <- e:
		case <-time.After(time.Second * 5):
			e.c <- fmt.Errorf("timeout 5s")
		}
		return e.c
	}

	srv := new(http.Server)
	srv.Handler = httpRoute(send)
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
