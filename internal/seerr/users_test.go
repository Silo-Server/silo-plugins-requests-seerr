package seerr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/httpclient"
)

func TestFindUserByEmailMatchesCaseInsensitive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"id":3,"email":"Bob@Example.com","permissions":32}]}`))
	}))
	defer srv.Close()
	u, err := FindUserByEmail(context.Background(), httpclient.New(srv.URL, "k", nil), "bob@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	if u == nil || u.ID != 3 {
		t.Fatalf("want user id 3, got %+v", u)
	}
}

func TestFindUserByEmailReturnsNilWhenAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"id":1,"email":"other@x.com"}]}`))
	}))
	defer srv.Close()
	u, err := FindUserByEmail(context.Background(), httpclient.New(srv.URL, "k", nil), "bob@example.com")
	if err != nil || u != nil {
		t.Fatalf("want nil user no error, got %+v / %v", u, err)
	}
}

func TestCreateUserSendsEmailAndPermissions(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":9,"email":"bob@example.com","permissions":160}`))
	}))
	defer srv.Close()
	u, err := CreateUser(context.Background(), httpclient.New(srv.URL, "k", nil), "bob@example.com", 160)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID != 9 {
		t.Fatalf("want id 9, got %+v", u)
	}
	if body["email"] != "bob@example.com" || int(body["permissions"].(float64)) != 160 {
		t.Fatalf("sent body wrong: %+v", body)
	}
}
