package server

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/valyala/fasthttp"
)

type Handler interface {
	http.Handler
	HandleFastHTTP(ctx *fasthttp.RequestCtx)
}

func NewHTTPStringHandler() Handler {
	return &httpStringHandler{NewCounter()}
}

type httpStringHandler struct {
	counter *Counter
}

func (h *httpStringHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/counter" {
		http.Error(w, "only POST /counter allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	v, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.counter.Bump(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = fmt.Fprintf(w, "%d", result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *httpStringHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	if string(ctx.Method()) != http.MethodPost || string(ctx.Path()) != "/counter" {
		ctx.Error("only POST /counter allowed", http.StatusMethodNotAllowed)
	}
	// TODO: Limit the size to be read
	v, err := strconv.ParseUint(string(ctx.Request.Body()), 10, 64)
	if err != nil {
		ctx.Error(err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.counter.Bump(v)
	if err != nil {
		ctx.Error(err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = fmt.Fprintf(ctx, "%d", result)
	if err != nil {
		ctx.Error(err.Error(), http.StatusInternalServerError)
		return
	}
}
