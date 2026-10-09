// Package api is the JSON HTTP API under /api/v1.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
)

const maxBodyBytes = 1 << 20

type handler struct{ svc *service.Service }

// Handler serves every /api/v1 route. Mount it at "/api/v1/".
func Handler(svc *service.Service) http.Handler {
	h := &handler{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/events", h.recordEvent)
	mux.HandleFunc("GET /api/v1/events", h.searchEvents)
	mux.HandleFunc("GET /api/v1/events/{id}", h.getEvent)
	mux.HandleFunc("GET /api/v1/systems", h.listSystems)
	mux.HandleFunc("GET /api/v1/systems/{system}/history", h.history)
	mux.HandleFunc("POST /api/v1/links", h.addLink)
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint: "+r.Method+" "+r.URL.Path, nil)
	})
	return mux
}

func (h *handler) recordEvent(w http.ResponseWriter, r *http.Request) {
	doc, err := decodeObject(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	ev, err := h.svc.Record(r.Context(), doc, service.MethodHuman)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/events/"+ev.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"event": ev})
}

func (h *handler) getEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		fail(w, r, fmt.Errorf("event %s: %w", id, err))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *handler) searchEvents(w http.ResponseWriter, r *http.Request) {
	p, err := service.SearchParamsFromQuery(r.URL.Query())
	if err != nil {
		fail(w, r, err)
		return
	}
	events, err := h.svc.Search(r.Context(), p)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *handler) listSystems(w http.ResponseWriter, r *http.Request) {
	systems, err := h.svc.Systems(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"systems": systems})
}

func (h *handler) history(w http.ResponseWriter, r *http.Request) {
	p, err := service.SearchParamsFromQuery(r.URL.Query())
	if err != nil {
		fail(w, r, err)
		return
	}
	system := r.PathValue("system")
	events, err := h.svc.History(r.Context(), system, p.Since, p.Limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"system": system, "events": events})
}

func (h *handler) addLink(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	var in service.LinkInput
	if err := dec.Decode(&in); err != nil {
		fail(w, r, &service.BadRequestError{Msg: "request body must be a JSON link object: " + err.Error()})
		return
	}
	l, err := h.svc.Link(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"link": l})
}

// decodeObject reads exactly one JSON object from the request body (at most
// 1 MiB). Numbers keep their precision.
func decodeObject(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, &service.BadRequestError{Msg: "request body must be a JSON object: " + err.Error()}
	}
	if doc == nil {
		return nil, &service.BadRequestError{Msg: "request body must be a JSON object"}
	}
	if dec.More() {
		return nil, &service.BadRequestError{Msg: "unexpected data after the JSON object"}
	}
	return doc, nil
}

type errorBody struct {
	Error struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details []event.Problem `json:"details,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string, details []event.Problem) {
	var b errorBody
	b.Error.Code, b.Error.Message, b.Error.Details = code, message, details
	writeJSON(w, status, b)
}

// fail maps a service error to the HTTP error contract.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *service.InvalidError
	var bad *service.BadRequestError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_event", "validation failed", invalid.Problems)
	case errors.As(err, &bad):
		writeError(w, http.StatusBadRequest, "bad_request", bad.Msg, nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
	default:
		slog.Error("api: internal error", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error", nil)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("api: write response", "err", err)
	}
}
