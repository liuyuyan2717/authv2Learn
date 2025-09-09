package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

func newOAuthServer() *server.Server {
	manager := manage.NewDefaultManager()

	// Access token generate method
	manager.MapAccessGenerate(generates.NewAccessGenerate())

	// Token expiration config
	manager.SetAuthorizeCodeTokenCfg(manage.DefaultAuthorizeCodeTokenCfg)
	manager.SetPasswordTokenCfg(&manage.Config{AccessTokenExp: time.Hour * 2})
	manager.SetClientTokenCfg(&manage.Config{AccessTokenExp: time.Hour * 2})

	// Token store: Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	// Test Redis connectivity
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
	log.Printf("redis connected: %s", "127.0.0.1:6379")

	// use custom redis token store with prefix myToken
	manager.MapTokenStorage(NewMyRedisTokenStore(rdb, "myToken"))
	//manager.MapTokenStorage()
	// Client store: in-memory for demo
	clientStore := store.NewClientStore()
	// Add a demo client: id=client_1 secret=secret_1
	_ = clientStore.Set("client_1", &models.Client{
		ID:     "client_1",
		Secret: "secret_1",
		Domain: "",
	})
	manager.MapClientStorage(clientStore)

	srv := server.NewServer(server.NewConfig(), manager)

	// Allowed grant types使用默认配置

	// Password authorization handler (simple demo: user=user, pass=pass)
	srv.SetPasswordAuthorizationHandler(func(ctx context.Context, clientID, username, password string) (userID string, err error) {
		if username == "user" && password == "pass" {
			return "user_1", nil
		}
		return "", errors.ErrAccessDenied
	})

	// Internal error handler
	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		log.Printf("internal error: %v", err)
		return
	})

	// Response error handler
	srv.SetResponseErrorHandler(func(re *errors.Response) {
		log.Printf("response error: %v", re.Error)
	})

	return srv
}

func main() {
	r := mux.NewRouter()
	srv := newOAuthServer()
	srv.SetClientInfoHandler(server.ClientFormHandler)
	// /token endpoint: issues tokens
	r.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {

		err := srv.HandleTokenRequest(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}).Methods(http.MethodPost)

	// /validate endpoint: validate access token (Bearer)
	r.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		ti, err := srv.ValidationBearerToken(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"active\":true,\"client_id\":\"" + ti.GetClientID() + "\",\"user_id\":\"" + ti.GetUserID() + "\"}"))
	}).Methods(http.MethodGet)

	addr := ":8080"
	log.Printf("OAuth2 server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
