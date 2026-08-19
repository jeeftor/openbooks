package server

import (
	"context"
	"log"
	"net/http"

	"github.com/google/uuid"
)

type uuidCtxKeyType string

const (
	uuidCtxKey uuidCtxKeyType = "user-uuid"
)

func (server *server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("OpenBooks")
		if err != nil {
			server.log.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		userUUID, err := uuid.Parse(cookie.Value)
		if err != nil {
			server.log.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), uuidCtxKey, userUUID)))
	})
}

func getUUID(ctx context.Context) uuid.UUID {
	uid, ok := ctx.Value(uuidCtxKey).(uuid.UUID)
	if !ok {
		log.Println("Unable to find user.")
	}
	return uid
}
