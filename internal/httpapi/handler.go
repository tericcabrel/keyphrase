// Package httpapi is the thin JSON-over-HTTP layer around the extractor
// and the page-text helper.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/webtext"
)

const (
	// MaxTextBytes is the limit on decoded text length.
	MaxTextBytes = 64 << 10
	// MaxBodyBytes bounds the raw request body. JSON escaping can expand
	// text up to six bytes per input byte ("\u0001"), so the cap leaves room
	// for a maximal escaped text plus the other fields.
	MaxBodyBytes = 6*MaxTextBytes + 16<<10
	// maxURLBodyBytes bounds a POST /v1/text body. A URL does not need the
	// keyword endpoint's escaped-text allowance.
	maxURLBodyBytes = 16 << 10
)

type request struct {
	Text     *string `json:"text"`
	Language *string `json:"language"`
	Limit    *int    `json:"limit"`
	Counts   *bool   `json:"counts"`
}

type response struct {
	Language string              `json:"language"`
	Keywords []extractor.Keyword `json:"keywords"`
	Counts   *extractor.Counts   `json:"counts,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type textRequest struct {
	URL *string `json:"url"`
}

type textResponse struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

// NewHandler returns the service mux: POST /v1/keywords, POST /v1/text, and GET /healthz.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/keywords", handleKeywords)
	mux.HandleFunc("POST /v1/text", handleText)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func handleKeywords(w http.ResponseWriter, r *http.Request) {
	body, status, err := decodeRequest(w, r)
	if err != nil {
		writeJSON(w, status, errorResponse{err.Error()})
		return
	}
	limit := extractor.DefaultLimit
	if body.Limit != nil {
		limit = *body.Limit
	}
	keywords, counts, err := extractor.ExtractWithCounts(*body.Text, *body.Language, limit)
	switch {
	case errors.Is(err, extractor.ErrUnsupportedLanguage):
		writeJSON(w, http.StatusBadRequest, errorResponse{fmt.Sprintf("unsupported language %q", *body.Language)})
		return
	case err != nil:
		writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
		return
	}
	result := response{Language: *body.Language, Keywords: keywords}
	if body.Counts != nil && *body.Counts {
		result.Counts = &counts
	}
	writeJSON(w, http.StatusOK, result)
}

func handleText(w http.ResponseWriter, r *http.Request) {
	rawURL, status, err := decodeTextRequest(w, r)
	if err != nil {
		writeJSON(w, status, errorResponse{err.Error()})
		return
	}
	text, truncated, err := webtext.Text(r.Context(), rawURL)
	if err != nil {
		if fetchErr, ok := errors.AsType[*webtext.Error](err); ok {
			writeJSON(w, fetchErr.StatusCode, errorResponse{fetchErr.Message})
			return
		}
		writeJSON(w, http.StatusBadGateway, errorResponse{"fetch failed"})
		return
	}
	writeJSON(w, http.StatusOK, textResponse{Text: text, Truncated: truncated})
}

func decodeRequest(w http.ResponseWriter, r *http.Request) (request, int, error) {
	var body request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err := decoder.Decode(&body); err != nil {
		if bodyTooLarge(err) {
			return body, http.StatusRequestEntityTooLarge, errBodyTooLarge
		}
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			return body, http.StatusBadRequest, fmt.Errorf("field %q has the wrong type", typeErr.Field)
		}
		return body, http.StatusBadRequest, errors.New("malformed JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if bodyTooLarge(err) {
			return body, http.StatusRequestEntityTooLarge, errBodyTooLarge
		}
		return body, http.StatusBadRequest, errors.New("request body must contain a single JSON object")
	}
	switch {
	case body.Language == nil:
		return body, http.StatusBadRequest, errors.New("language is required")
	case body.Text == nil:
		return body, http.StatusBadRequest, errors.New("text is required")
	case len(*body.Text) > MaxTextBytes:
		return body, http.StatusRequestEntityTooLarge, fmt.Errorf("text exceeds %d bytes", MaxTextBytes)
	case body.Limit != nil && *body.Limit < 1:
		return body, http.StatusBadRequest, extractor.ErrInvalidLimit
	}
	return body, 0, nil
}

func decodeTextRequest(w http.ResponseWriter, r *http.Request) (string, int, error) {
	var body textRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxURLBodyBytes))
	if err := decoder.Decode(&body); err != nil {
		if bodyTooLarge(err) {
			return "", http.StatusRequestEntityTooLarge, fmt.Errorf("request body exceeds %d bytes", maxURLBodyBytes)
		}
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			return "", http.StatusBadRequest, fmt.Errorf("field %q has the wrong type", typeErr.Field)
		}
		return "", http.StatusBadRequest, errors.New("malformed JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if bodyTooLarge(err) {
			return "", http.StatusRequestEntityTooLarge, fmt.Errorf("request body exceeds %d bytes", maxURLBodyBytes)
		}
		return "", http.StatusBadRequest, errors.New("request body must contain a single JSON object")
	}
	if body.URL == nil {
		return "", http.StatusBadRequest, errors.New("url is required")
	}
	return *body.URL, 0, nil
}

var errBodyTooLarge = fmt.Errorf("request body exceeds %d bytes", MaxBodyBytes)

func bodyTooLarge(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return ok
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
