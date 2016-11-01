package tinyjail

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/skoo87/tinyjail/proto"
)

func httpRoute(send func(event) chan interface{}) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/hello", func(w http.ResponseWriter, r *http.Request) {
		ev := event{
			action: "hello",
		}
		json.NewEncoder(w).Encode(<-send(ev))
	})

	// '/v1/start/name' start init process.
	mux.HandleFunc("/v1/start", func(w http.ResponseWriter, r *http.Request) {

	})

	// '/v1/stop/name' stop init process.
	mux.HandleFunc("/v1/stop", func(w http.ResponseWriter, r *http.Request) {

	})

	// '/v1/delete/name' stop master process and delete container.
	mux.HandleFunc("/v1/delete", func(w http.ResponseWriter, r *http.Request) {

	})

	// '/v1/exec/name' execute process in container.
	mux.HandleFunc("/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		req := &proto.ExecRequest{}

		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			errorln("Decode exec request error: %v", err)

			errInfo := proto.ExecResponse{}
			errInfo.Status = "failed"
			errInfo.Desc = err.Error()

			// TODO handle err
			json.NewEncoder(w).Encode(errInfo)
			return
		}

		ev := event{
			action: evExec,
			data:   req,
		}
		json.NewEncoder(w).Encode(<-send(ev))
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
			e.c <- proto.Response{
				Status: "failed",
				Desc:   "timeout 5s",
			}
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
