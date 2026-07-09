package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/Silo-Community/silo-plugins-requests-seerr/internal/seerr"
)

// seerrStub records POST bodies and serves canned create responses.
type seerrStub struct {
	mu     sync.Mutex
	bodies []map[string]any
}

func (s *seerrStub) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost {
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			s.mu.Lock()
			s.bodies = append(s.bodies, b)
			n := len(s.bodies)
			s.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			// distinct ids so we can tell HD/4K targets apart
			_, _ = w.Write([]byte(`{"id":` + itoa(100+n) + `,"status":2,"media":{"status":2}}`))
			return
		}
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	})
}

func conn(t *testing.T, id, baseURL string, supports4k bool) *pluginv1.RouterConnection {
	cfg, err := structpb.NewStruct(map[string]any{"supports_4k": supports4k})
	if err != nil {
		t.Fatalf("structpb: %v", err)
	}
	return &pluginv1.RouterConnection{Id: id, BaseUrl: baseURL, ApiKey: "k", Config: cfg}
}

func TestFulfillHDOnly(t *testing.T) {
	stub := &seerrStub{}
	srv := httptest.NewServer(stub.handler(t))
	defer srv.Close()

	resp, err := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, false)},
	})
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if len(resp.GetTargets()) != 1 {
		t.Fatalf("want 1 target, got %d msg=%q", len(resp.GetTargets()), resp.GetMessage())
	}
	tgt := resp.GetTargets()[0]
	if tgt.GetStatus() != "queued" || tgt.GetQuality() != "1080p" || tgt.GetExternalId() != "101" {
		t.Fatalf("bad target: %+v", tgt)
	}
	if stub.bodies[0]["is4k"] != false || stub.bodies[0]["mediaType"] != "movie" || stub.bodies[0]["mediaId"] != float64(42) {
		t.Fatalf("bad body: %+v", stub.bodies[0])
	}
	if _, ok := stub.bodies[0]["seasons"]; ok {
		t.Fatalf("movie body must not carry seasons: %+v", stub.bodies[0])
	}
}

func TestFulfillHDPlus4KWhenSupported(t *testing.T) {
	stub := &seerrStub{}
	srv := httptest.NewServer(stub.handler(t))
	defer srv.Close()

	resp, _ := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "series", ExternalIds: map[string]string{"tmdb": "9"}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}, {Id: "2160p", Is4K: true}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, true)},
	})
	if len(resp.GetTargets()) != 2 {
		t.Fatalf("want 2 targets, got %d", len(resp.GetTargets()))
	}
	if stub.bodies[0]["is4k"] != false || stub.bodies[1]["is4k"] != true {
		t.Fatalf("is4k mapping wrong: %+v %+v", stub.bodies[0], stub.bodies[1])
	}
	if stub.bodies[0]["mediaType"] != "tv" || stub.bodies[0]["seasons"] != "all" {
		t.Fatalf("series should map to tv + seasons all: %+v", stub.bodies[0])
	}
}

func TestFulfill4KSkippedWhenUnsupported(t *testing.T) {
	stub := &seerrStub{}
	srv := httptest.NewServer(stub.handler(t))
	defer srv.Close()

	resp, _ := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "9"}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}, {Id: "2160p", Is4K: true}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, false)},
	})
	if len(resp.GetTargets()) != 1 || resp.GetTargets()[0].GetQuality() != "1080p" {
		t.Fatalf("4k should be skipped, got %d targets", len(resp.GetTargets()))
	}
	if len(stub.bodies) != 1 {
		t.Fatalf("only the HD request should be sent, got %d", len(stub.bodies))
	}
}

func TestFulfillMissingTMDBReturnsMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()
	resp, _ := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, false)},
	})
	if len(resp.GetTargets()) != 0 || resp.GetMessage() == "" {
		t.Fatalf("missing tmdb should yield zero targets + a request-level message, got %d targets msg=%q", len(resp.GetTargets()), resp.GetMessage())
	}
}

func TestFulfillDuplicateRecoversID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"message":"already requested"}`))
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodGet:
			w.Write([]byte(`{"results":[{"id":314,"is4k":false,"media":{"tmdbId":42}}]}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	resp, _ := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, false)},
	})
	tgt := resp.GetTargets()[0]
	if tgt.GetStatus() != "queued" || tgt.GetExternalId() != "314" {
		t.Fatalf("409 should recover the existing id as queued: %+v", tgt)
	}
}

func TestFulfillEmptyBodyRecoversID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated) // 201 with an empty body: no usable id
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodGet:
			w.Write([]byte(`{"results":[{"id":271,"is4k":false,"media":{"tmdbId":42,"status":3}}]}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	resp, _ := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, false)},
	})
	if len(resp.GetTargets()) != 1 {
		t.Fatalf("want 1 target, got %d", len(resp.GetTargets()))
	}
	tgt := resp.GetTargets()[0]
	if tgt.GetStatus() != "queued" || tgt.GetExternalId() != "271" {
		t.Fatalf("empty-body create should recover the existing id as queued: %+v", tgt)
	}
}

func TestFulfillZeroTargetsReturnsMessage(t *testing.T) {
	resp, _ := New().Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "2160p", Is4K: true}},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", "http://unused", false)},
	})
	if len(resp.GetTargets()) != 0 || resp.GetMessage() == "" {
		t.Fatalf("want zero targets + a message, got %d targets msg=%q", len(resp.GetTargets()), resp.GetMessage())
	}
}

func TestCheckStatusMapsAndSkipsMissingConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// request 5 -> processing (downloading); request 6 -> available (completed)
		switch r.URL.Path {
		case "/api/v1/request/5":
			w.Write([]byte(`{"id":5,"status":2,"media":{"status":3}}`))
		case "/api/v1/request/6":
			w.Write([]byte(`{"id":6,"status":2,"media":{"status":5}}`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	resp, err := New().CheckStatus(context.Background(), &pluginv1.CheckStatusRequest{
		Request: &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "1"}},
		Targets: []*pluginv1.TargetRef{
			{Quality: "1080p", ConnectionId: "c1", ExternalId: "5"},
			{Quality: "2160p", ConnectionId: "c1", ExternalId: "6"},
			{Quality: "1080p", ConnectionId: "missing", ExternalId: "9"}, // connection not provided -> skipped
		},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, true)},
	})
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if len(resp.GetStatuses()) != 2 {
		t.Fatalf("want 2 statuses (missing connection skipped), got %d", len(resp.GetStatuses()))
	}
	got := map[string]string{}
	for _, st := range resp.GetStatuses() {
		got[st.GetQuality()] = st.GetStatus()
	}
	if got["1080p"] != "downloading" || got["2160p"] != "completed" {
		t.Fatalf("status mapping wrong: %+v", got)
	}
}

func TestCheckStatus404MapsToFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/request/77" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"not found"}`))
			return
		}
		http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
	}))
	defer srv.Close()

	resp, err := New().CheckStatus(context.Background(), &pluginv1.CheckStatusRequest{
		Request: &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "1"}},
		Targets: []*pluginv1.TargetRef{
			{Quality: "1080p", ConnectionId: "c1", ExternalId: "77"},
		},
		Connections: []*pluginv1.RouterConnection{conn(t, "c1", srv.URL, false)},
	})
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if len(resp.GetStatuses()) != 1 {
		t.Fatalf("404 should yield a failed status, not a skip: got %d statuses", len(resp.GetStatuses()))
	}
	st := resp.GetStatuses()[0]
	if st.GetStatus() != "failed" || st.GetMessage() == "" {
		t.Fatalf("404 should map to failed with a message, got %+v", st)
	}
}

func TestTestConnectionUsesAuthMe(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/me" {
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"id":1,"email":"owner@example.com"}`))
	}))
	defer ok.Close()
	res, _ := New().TestConnection(context.Background(), &pluginv1.TestConnectionRequest{
		Connection: conn(t, "c1", ok.URL, false),
	})
	if !res.GetOk() {
		t.Fatalf("want ok, got %+v", res)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer bad.Close()
	res, _ = New().TestConnection(context.Background(), &pluginv1.TestConnectionRequest{
		Connection: conn(t, "c1", bad.URL, false),
	})
	if res.GetOk() || res.GetMessage() == "" {
		t.Fatalf("want not-ok with message, got %+v", res)
	}

	// A 200 from a login wall / proxy with a non-user body must NOT pass.
	wall := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html>login</html>`))
	}))
	defer wall.Close()
	res, _ = New().TestConnection(context.Background(), &pluginv1.TestConnectionRequest{
		Connection: conn(t, "c1", wall.URL, false),
	})
	if res.GetOk() || res.GetMessage() == "" {
		t.Fatalf("want not-ok for 200 login-wall body, got %+v", res)
	}
}

func TestFulfillMappedUsesExistingSeerrUser(t *testing.T) {
	var createCalled bool
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodGet:
			w.Write([]byte(`{"results":[{"id":7,"email":"bob@example.com","permissions":32}]}`))
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodPost:
			createCalled = true
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":99}`))
		case r.URL.Path == "/api/v1/request":
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":55,"status":2,"media":{"status":3,"tmdbId":42}}`))
		}
	}))
	defer srv.Close()

	cfg, _ := structpb.NewStruct(map[string]any{"requester_mode": "mapped"})
	resp, err := (&Server{}).Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}, RequesterEmail: "bob@example.com"},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p", Is4K: false}},
		Connections: []*pluginv1.RouterConnection{{Id: "c1", BaseUrl: srv.URL, ApiKey: "k", Config: cfg}},
	})
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if createCalled {
		t.Fatalf("must reuse existing user, not create")
	}
	if int(reqBody["userId"].(float64)) != 7 {
		t.Fatalf("request userId = %v, want 7", reqBody["userId"])
	}
	if len(resp.GetTargets()) != 1 || resp.GetTargets()[0].GetStatus() != "queued" {
		t.Fatalf("targets = %+v", resp.GetTargets())
	}
}

func newUserCreateStub(t *testing.T, createBody *map[string]any, createResp string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodGet:
			w.Write([]byte(`{"results":[]}`)) // no existing user -> create
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(createBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(createResp))
		case r.URL.Path == "/api/v1/request":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":55,"status":2,"media":{"status":3,"tmdbId":42}}`))
		}
	}))
}

func mustFulfill(t *testing.T, baseURL string, cfg *structpb.Struct, qs []*pluginv1.RequestedQuality) {
	t.Helper()
	if _, err := (&Server{}).Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}, RequesterEmail: "new@example.com"},
		Qualities:   qs,
		Connections: []*pluginv1.RouterConnection{{Id: "c1", BaseUrl: baseURL, ApiKey: "k", Config: cfg}},
	}); err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
}

func TestFulfillMappedCreatesMissingUserWithPermissions(t *testing.T) {
	var createBody map[string]any
	srv := newUserCreateStub(t, &createBody, `{"id":99,"email":"new@example.com"}`)
	defer srv.Close()

	cfg, _ := structpb.NewStruct(map[string]any{"requester_mode": "mapped", "auto_approve": true})
	mustFulfill(t, srv.URL, cfg, []*pluginv1.RequestedQuality{{Id: "1080p"}})

	if createBody["email"] != "new@example.com" {
		t.Fatalf("create email = %v", createBody["email"])
	}
	if got := int(createBody["permissions"].(float64)); got != seerr.PermRequest|seerr.PermAutoApprove {
		t.Fatalf("permissions = %d, want %d (REQUEST|AUTO_APPROVE)", got, seerr.PermRequest|seerr.PermAutoApprove)
	}
}

func TestFulfillMappedGrants4KWhenRequestHas4K(t *testing.T) {
	var createBody map[string]any
	srv := newUserCreateStub(t, &createBody, `{"id":99}`)
	defer srv.Close()

	cfg, _ := structpb.NewStruct(map[string]any{"requester_mode": "mapped", "auto_approve": true, "supports_4k": true})
	mustFulfill(t, srv.URL, cfg, []*pluginv1.RequestedQuality{{Id: "1080p"}, {Id: "2160p", Is4K: true}})

	want := seerr.PermRequest | seerr.PermRequest4K | seerr.PermAutoApprove | seerr.PermAutoApprove4K
	if got := int(createBody["permissions"].(float64)); got != want {
		t.Fatalf("permissions = %d, want %d (incl 4K)", got, want)
	}
}


func TestFulfillMappedAutoApproveOffGrantsRequestOnly(t *testing.T) {
	var createBody map[string]any
	srv := newUserCreateStub(t, &createBody, `{"id":99}`)
	defer srv.Close()

	cfg, _ := structpb.NewStruct(map[string]any{"requester_mode": "mapped"})
	mustFulfill(t, srv.URL, cfg, []*pluginv1.RequestedQuality{{Id: "1080p"}})

	if got := int(createBody["permissions"].(float64)); got != seerr.PermRequest {
		t.Fatalf("permissions = %d, want %d (REQUEST only)", got, seerr.PermRequest)
	}
}

func TestFulfillFallsBackToAdminOnResolveFailure(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/v1/request":
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":55,"status":2,"media":{"status":3,"tmdbId":42}}`))
		}
	}))
	defer srv.Close()

	cfg, _ := structpb.NewStruct(map[string]any{"requester_mode": "mapped"})
	_, err := (&Server{}).Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}, RequesterEmail: "bob@example.com"},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p"}},
		Connections: []*pluginv1.RouterConnection{{Id: "c1", BaseUrl: srv.URL, ApiKey: "k", Config: cfg}},
	})
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if _, ok := reqBody["userId"]; ok {
		t.Fatalf("admin fallback must omit userId, got %v", reqBody["userId"])
	}
}

func TestFulfillRequireMappedUserFailsWhenUnmapped(t *testing.T) {
	requestCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodGet:
			w.Write([]byte(`{"results":[]}`))
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/v1/request":
			requestCalled = true
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	cfg, _ := structpb.NewStruct(map[string]any{"requester_mode": "mapped", "require_mapped_user": true})
	resp, err := (&Server{}).Fulfill(context.Background(), &pluginv1.FulfillRequest{
		Request:     &pluginv1.RequestDescriptor{MediaType: "movie", ExternalIds: map[string]string{"tmdb": "42"}, RequesterEmail: "bob@example.com"},
		Qualities:   []*pluginv1.RequestedQuality{{Id: "1080p"}},
		Connections: []*pluginv1.RouterConnection{{Id: "c1", BaseUrl: srv.URL, ApiKey: "k", Config: cfg}},
	})
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if requestCalled {
		t.Fatalf("require_mapped_user on: must NOT submit the request when the user can't be mapped")
	}
	if resp.GetMessage() == "" || len(resp.GetTargets()) != 0 {
		t.Fatalf("want a request-level failure message and no targets, got msg=%q targets=%d", resp.GetMessage(), len(resp.GetTargets()))
	}
}

func TestListConfigOptionsAndValidateAreEmpty(t *testing.T) {
	opts, err := New().ListConfigOptions(context.Background(), &pluginv1.ListConfigOptionsRequest{
		Connection: conn(t, "c1", "http://s", false),
	})
	if err != nil || len(opts.GetOptionsByField()) != 0 {
		t.Fatalf("want empty options, got %+v err=%v", opts.GetOptionsByField(), err)
	}
	val, err := New().Validate(context.Background(), &pluginv1.ValidateRequest{
		Connection: conn(t, "c1", "http://s", false),
	})
	if err != nil || len(val.GetFieldErrors()) != 0 || val.GetFormError() != "" {
		t.Fatalf("want empty validate, got %+v err=%v", val, err)
	}
}
