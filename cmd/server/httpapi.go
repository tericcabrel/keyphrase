package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tericcabrel/keyphrase/extractor"
)

const (
	// maxTextBytes is the limit on decoded text length.
	maxTextBytes = 64 << 10
	// maxBodyBytes bounds the raw request body. JSON escaping can expand
	// text up to six bytes per input byte ("\u0001"), so the cap leaves room
	// for a maximal escaped text plus the other fields.
	maxBodyBytes = 6*maxTextBytes + 16<<10
)

type keywordRequest struct {
	Text              *string  `json:"text"`
	Language          *string  `json:"language"`
	Limit             *int     `json:"limit"`
	Counts            *bool    `json:"counts"`
	DropLoneNouns     *bool    `json:"drop_lone_nouns"`
	LoneNounMinRarity *float64 `json:"lone_noun_min_rarity"`
	DropOneOffPhrases *bool    `json:"drop_one_off_phrases"`
	OneOffMinOthers   *int     `json:"one_off_min_others"`
}

type keywordResponse struct {
	Language string              `json:"language"`
	Keywords []extractor.Keyword `json:"keywords"`
	Counts   *extractor.Counts   `json:"counts,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/keywords", handleKeywords)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	handler := http.Handler(mux)
	if token := os.Getenv("KEYPHRASE_TOKEN"); token != "" {
		handler = requireBearerToken(token, handler)
	}

	return handler
}

func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)

			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})

			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleKeywords(w http.ResponseWriter, r *http.Request) {
	body, status, err := decodeKeywordRequest(w, r)
	if err != nil {
		writeJSON(w, status, errorResponse{err.Error()})
		return
	}
	limit := extractor.DefaultLimit
	if body.Limit != nil {
		limit = *body.Limit
	}
	options := extractOptionsFromRequest(body)
	keywords, counts, err := extractor.Extract(*body.Text, *body.Language, limit, options)
	switch {
	case errors.Is(err, extractor.ErrUnsupportedLanguage):
		writeJSON(w, http.StatusBadRequest, errorResponse{fmt.Sprintf("unsupported language %q", *body.Language)})
		return
	case err != nil:
		writeJSON(w, http.StatusBadRequest, errorResponse{err.Error()})
		return
	}
	wantCounts := body.Counts == nil || *body.Counts
	result := keywordResponse{Language: *body.Language, Keywords: keywords}
	if wantCounts {
		result.Counts = &counts
	}
	writeJSON(w, http.StatusOK, result)
}

func extractOptionsFromRequest(body keywordRequest) extractor.ExtractOptions {
	options := extractor.DefaultExtractOptions
	if body.DropLoneNouns != nil {
		options.DropLoneNouns = *body.DropLoneNouns
	}
	if body.LoneNounMinRarity != nil {
		options.LoneNounMinRarity = *body.LoneNounMinRarity
	}
	if body.DropOneOffPhrases != nil {
		options.DropOneOffPhrases = *body.DropOneOffPhrases
	}
	if body.OneOffMinOthers != nil {
		options.OneOffMinOthers = *body.OneOffMinOthers
	}

	return options
}

func decodeKeywordRequest(w http.ResponseWriter, r *http.Request) (keywordRequest, int, error) {
	var body keywordRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
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
	case len(*body.Text) > maxTextBytes:
		return body, http.StatusRequestEntityTooLarge, fmt.Errorf("text exceeds %d bytes", maxTextBytes)
	case body.Limit != nil && *body.Limit < 1:
		return body, http.StatusBadRequest, extractor.ErrInvalidLimit
	}

	return body, 0, nil
}

var errBodyTooLarge = fmt.Errorf("request body exceeds %d bytes", maxBodyBytes)

func bodyTooLarge(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return ok
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
