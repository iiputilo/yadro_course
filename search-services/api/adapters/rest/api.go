package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"yadro.com/course/api/core"
)

type pingReply struct {
	Replies map[string]string `json:"replies"`
}

type statsReply struct {
	WordsTotal    int `json:"words_total"`
	WordsUnique   int `json:"words_unique"`
	ComicsFetched int `json:"comics_fetched"`
	ComicsTotal   int `json:"comics_total"`
}

type statusReply struct {
	Status string `json:"status"`
}

type comicsReply struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type searchReply struct {
	Comics []comicsReply `json:"comics"`
	Total  int           `json:"total"`
}

type loginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type authService interface {
	IssueToken() (string, error)
}

const DefaultLimit = 10

func ParseLimit(s string) (int, error) {
	if s == "" {
		return DefaultLimit, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0, core.ErrBadLimit
	}
	return v, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func NewPingHandler(log *slog.Logger, pingers map[string]core.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := pingReply{Replies: map[string]string{}}
		for name, p := range pingers {
			if err := p.Ping(r.Context()); err != nil {
				log.Warn("ping failed", "service", name, "error", err)
				resp.Replies[name] = "unavailable"
				continue
			}
			resp.Replies[name] = "ok"
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func NewUpdateHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := updater.Update(r.Context())
		if err != nil {
			switch {
			case errors.Is(err, core.ErrAlreadyExists):
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "already_running"})
			case errors.Is(err, core.ErrBadArguments):
				http.Error(w, "bad request", http.StatusBadRequest)
			default:
				log.Error("update failed", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func NewUpdateStatsHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := updater.Stats(r.Context())
		if err != nil {
			log.Error("stats failed", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, statsReply{
			WordsTotal:    st.WordsTotal,
			WordsUnique:   st.WordsUnique,
			ComicsFetched: st.ComicsFetched,
			ComicsTotal:   st.ComicsTotal,
		})
	}
}

func NewUpdateStatusHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := updater.Status(r.Context())
		if err != nil {
			log.Error("status failed", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, statusReply{Status: string(st)})
	}
}

func NewDropHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := updater.Drop(r.Context()); err != nil {
			log.Error("drop failed", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func NewSearchHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		phrase := r.URL.Query().Get("phrase")
		limitStr := r.URL.Query().Get("limit")

		limit, err := ParseLimit(limitStr)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if phrase == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		res, err := searcher.Search(r.Context(), phrase, limit)
		if err != nil {
			switch {
			case errors.Is(err, core.ErrBadPhrase), errors.Is(err, core.ErrBadLimit):
				http.Error(w, "bad request", http.StatusBadRequest)
			default:
				log.Error("search failed", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		out := searchReply{
			Comics: make([]comicsReply, 0, len(res.Comics)),
			Total:  res.Total,
		}
		for _, c := range res.Comics {
			out.Comics = append(out.Comics, comicsReply{
				ID:  c.ID,
				URL: c.URL,
			})
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func NewISearchHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		phrase := r.URL.Query().Get("phrase")
		limitStr := r.URL.Query().Get("limit")

		limit, err := ParseLimit(limitStr)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if phrase == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		res, err := searcher.ISearch(r.Context(), phrase, limit)
		if err != nil {
			switch {
			case errors.Is(err, core.ErrBadPhrase), errors.Is(err, core.ErrBadLimit):
				http.Error(w, "bad request", http.StatusBadRequest)
			default:
				log.Error("search failed", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		out := searchReply{
			Comics: make([]comicsReply, 0, len(res.Comics)),
			Total:  res.Total,
		}
		for _, c := range res.Comics {
			out.Comics = append(out.Comics, comicsReply{
				ID:  c.ID,
				URL: c.URL,
			})
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func NewLoginHandler(log *slog.Logger, auth authService, adminUser, adminPass string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Name != adminUser || req.Password != adminPass {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		token, err := auth.IssueToken()
		if err != nil {
			log.Error("failed to issue token", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(token))
	}
}
