package wss

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var testSecret = []byte("service-secret")

func newTestWSS(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := newEngine(ut.EnvConfig{
		AuthConfig:  ut.AuthConfig{ServiceSecretKey: testSecret},
		PeersConfig: ut.PeersConfig{WssLogsPath: t.TempDir()}, // no trailing slash: joined, not concatenated
	})
	ts := httptest.NewServer(engine)
	t.Cleanup(ts.Close)

	return ts
}

// dial opens a session; it returns the HTTP status when the upgrade is refused.
func dial(t *testing.T, ts *httptest.Server, query url.Values, header http.Header) (*websocket.Conn, int) {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + "/get-session?" + query.Encode()
	conn, resp, err := websocket.DefaultDialer.Dial(u, header)
	if err != nil {
		if resp == nil {
			t.Fatalf("dial: %v", err)
		}

		return nil, resp.StatusCode
	}
	t.Cleanup(func() { _ = conn.Close() })

	return conn, http.StatusSwitchingProtocols
}

func ticket(jid, role string) string {
	return ut.SignWSTicket(testSecret, jid, role, "1001", time.Now())
}

func TestSessionAuthorization(t *testing.T) {
	ts := newTestWSS(t)
	service := http.Header{"X-Service-Secret": {string(testSecret)}}

	cases := []struct {
		name   string
		query  url.Values
		header http.Header
		want   int
	}{
		{"consumer without a ticket", url.Values{"jid": {"1"}, "role": {"consumer"}}, nil, http.StatusUnauthorized},
		{"ticket for another job", url.Values{"jid": {"2"}, "role": {"consumer"}, "ticket": {ticket("1", "consumer")}}, nil, http.StatusUnauthorized},
		{"consumer ticket used as jack", url.Values{"jid": {"1"}, "role": {"jack"}, "ticket": {ticket("1", "consumer")}}, nil, http.StatusUnauthorized},
		{"producer without the service secret", url.Values{"jid": {"1"}, "role": {"producer"}}, nil, http.StatusUnauthorized},
		{"producer with a wrong secret", url.Values{"jid": {"1"}, "role": {"producer"}}, http.Header{"X-Service-Secret": {"nope"}}, http.StatusUnauthorized},
		{"unknown role", url.Values{"jid": {"1"}, "role": {"admin"}}, service, http.StatusBadRequest},
		{"missing jid", url.Values{"role": {"producer"}}, service, http.StatusBadRequest},
		{"browser from another site", url.Values{"jid": {"1"}, "role": {"consumer"}, "ticket": {ticket("1", "consumer")}},
			http.Header{"Origin": {"https://evil.example"}}, http.StatusForbidden},
		{"consumer with a ticket", url.Values{"jid": {"1"}, "role": {"consumer"}, "ticket": {ticket("1", "consumer")}}, nil, http.StatusSwitchingProtocols},
		{"producer with the service secret", url.Values{"jid": {"1"}, "role": {"producer"}}, service, http.StatusSwitchingProtocols},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := dial(t, ts, tc.query, tc.header); got != tc.want {
				t.Errorf("status %d, want %d", got, tc.want)
			}
		})
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/delete-session?jid=1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("delete-session without the secret: %d, want 401", resp.StatusCode)
	}
}

func TestProducerReachesConsumers(t *testing.T) {
	ts := newTestWSS(t)
	const jid = "42"

	var consumers []*websocket.Conn
	for range 2 {
		c, code := dial(t, ts, url.Values{"jid": {jid}, "role": {"consumer"}, "ticket": {ticket(jid, "consumer")}}, nil)
		if code != http.StatusSwitchingProtocols {
			t.Fatalf("consumer refused: %d", code)
		}
		consumers = append(consumers, c)
	}
	other, _ := dial(t, ts, url.Values{"jid": {"43"}, "role": {"consumer"}, "ticket": {ticket("43", "consumer")}}, nil)
	producer, code := dial(t, ts, url.Values{"jid": {jid}, "role": {"producer"}}, http.Header{"X-Service-Secret": {string(testSecret)}})
	if code != http.StatusSwitchingProtocols {
		t.Fatalf("producer refused: %d", code)
	}

	waitForClients(t, jid, 1, 2)
	if err := producer.WriteMessage(websocket.TextMessage, []byte("line 1")); err != nil {
		t.Fatal(err)
	}
	for i, c := range consumers {
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, msg, err := c.ReadMessage(); err != nil || string(msg) != "line 1" {
			t.Fatalf("consumer %d: %q, %v", i, msg, err)
		}
	}

	// consumers only listen: what they send is not broadcast
	if err := consumers[0].WriteMessage(websocket.TextMessage, []byte("injected")); err != nil {
		t.Fatal(err)
	}
	_ = consumers[1].SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		_, msg, err := consumers[1].ReadMessage()
		if err != nil {
			break
		}
		if string(msg) == "injected" {
			t.Fatal("a consumer's message was broadcast")
		}
	}

	// sessions are per job
	_ = other.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, msg, err := other.ReadMessage(); err == nil {
		t.Errorf("job 43's consumer got job 42's output: %q", msg)
	}
}

// waitForClients waits until job jid's session has registered the clients
// (registration runs on the session's goroutine, after the upgrade).
func waitForClients(t *testing.T, jid string, producers, consumers int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		registry.Lock()
		s := registry.servers[jid]
		registry.Unlock()
		if s == nil {
			continue
		}
		s.Lock()
		ok := len(s.Producers) == producers && len(s.Consumers) == consumers
		s.Unlock()
		if ok {
			return
		}
	}
	t.Fatalf("session %s never had %d producer(s) and %d consumer(s)", jid, producers, consumers)
}
