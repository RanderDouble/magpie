package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A saved account's refresh refused because another copy of the sign-in
// refreshed it first (refresh_token_reused): Codex is signed in to the same
// login in another folder with the token the family went on with, so the
// sign-in is renewed there and the copy takes it, with that folder, rather
// than left marked expired.
func TestCodexRefreshAdoptsTheFolderItWentOnIn(t *testing.T) {
	home := signIn(t)
	t.Setenv("CODEX_HOME", "")
	work := filepath.Join(home, ".codex-work")
	fresh := fakeJWT(map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix()), "n": "work-2"})
	idTok := fakeJWT(map[string]any{"email": "work@example.com",
		"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "plus", "chatgpt_account_id": "acct-work"}})
	writeFile(t, filepath.Join(work, "auth.json"), map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{"id_token": idTok, "access_token": fresh,
			"refresh_token": "r-work-2", "account_id": "acct-work"},
	})
	// magpie's copy of it, with the refresh token the family has left behind
	dead := codexAuthBytes(t, idTok, "r-work")
	if err := writeLogins([]savedLogin{{Agent: "codex", User: "work@example.com", Plan: "plus", Auth: dead}}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var asked []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		asked = append(asked, body["refresh_token"])
		mu.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"refresh_token_reused"}`)
	}))
	defer fake.Close()
	oldTok := codexTokenURL
	t.Cleanup(func() { codexTokenURL = oldTok })
	codexTokenURL = fake.URL
	ForgetAccounts()

	tok, _, err := savedLoginToken(context.Background(), "codex", "work@example.com")
	if err != nil || tok != fresh {
		t.Fatalf("renewed to %q, %v; want the folder's %q", tok, err, fresh)
	}
	mu.Lock()
	if len(asked) != 1 || asked[0] != "r-work" {
		t.Errorf("refreshed with %v, want the copy's spent r-work once", asked)
	}
	mu.Unlock()
	copy1 := savedLoginOf(t, "work@example.com")
	var a codexAuth
	if json.Unmarshal(copy1.Auth, &a) != nil || a.Tokens.RefreshToken != "r-work-2" || a.Tokens.AccessToken != fresh {
		t.Fatalf("the copy didn't take the folder's sign-in: %s", copy1.Auth)
	}
	if copy1.Home != work || copy1.Lapsed != "" {
		t.Fatalf("home %q, lapsed %q; want %q and none", copy1.Home, copy1.Lapsed, work)
	}
	// the next refresh goes to that folder, not to the copy
	if tok, _, err = savedLoginToken(context.Background(), "codex", "work@example.com"); err != nil || tok != fresh {
		t.Fatalf("the next renew: %q, %v", tok, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 1 {
		t.Errorf("the next renew asked the token endpoint: %v", asked)
	}
}

// A refused refresh says which refusal it was: a token another copy of the
// sign-in spent (refresh_token_reused), or one that is too old.
func TestCodexRefreshRefusalSaysWhich(t *testing.T) {
	for _, c := range []struct {
		code string
		want string
	}{
		{"refresh_token_reused", "was refreshed elsewhere first — add the account again to use it"},
		{"refresh_token_expired", "has expired — add the account again to use it"},
	} {
		t.Run(c.code, func(t *testing.T) {
			signIn(t)
			t.Setenv("CODEX_HOME", "")
			idTok := fakeJWT(map[string]any{"email": "gone@example.com",
				"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "plus", "chatgpt_account_id": "acct-gone"}})
			dead := codexAuthBytes(t, idTok, "r-gone")
			if err := writeLogins([]savedLogin{{Agent: "codex", User: "gone@example.com", Plan: "plus", Auth: dead}}); err != nil {
				t.Fatal(err)
			}
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				io.WriteString(w, `{"error":"`+c.code+`"}`)
			}))
			defer fake.Close()
			oldTok := codexTokenURL
			t.Cleanup(func() { codexTokenURL = oldTok })
			codexTokenURL = fake.URL
			ForgetAccounts()

			_, _, err := savedLoginToken(context.Background(), "codex", "gone@example.com")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("refused with %v, want %q", err, c.want)
			}
			if l := savedLoginOf(t, "gone@example.com").Lapsed; !strings.Contains(l, c.want) {
				t.Fatalf("the account says %q, want %q", l, c.want)
			}
		})
	}
}

// codexAuthBytes is an auth.json as Codex writes it, its access token past
// its expiry, holding refresh.
func codexAuthBytes(t *testing.T, idTok, refresh string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{"auth_mode": "chatgpt",
		"tokens": map[string]any{"id_token": idTok, "refresh_token": refresh, "account_id": "acct-gone",
			"access_token": fakeJWT(map[string]any{"exp": float64(time.Now().Add(-time.Hour).Unix())})}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func savedLoginOf(t *testing.T, user string) savedLogin {
	t.Helper()
	for _, l := range readLogins() {
		if l.Agent == "codex" && strings.EqualFold(l.User, user) {
			return l
		}
	}
	t.Fatalf("no saved codex account %q in %+v", user, readLogins())
	return savedLogin{}
}
