package provider

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Codex signed in with an API key here, the saved accounts aren't refused
// any more: one stands in, as a saved Claude account does while Claude
// Code is signed out. The provider is listed on it, and the account the
// app-out takes as the one Codex is on.
func TestCodexStandIn(t *testing.T) {
	shellFakes(t)
	home := signIn(t)
	rememberLogins(true)
	codexSignIn(t, home, "spare@example.com", "r-spare")
	rememberLogins(true)
	// the API-key auth.json the reporter had: no ChatGPT sign-in here
	writeFile(t, filepath.Join(home, ".codex", "auth.json"), map[string]any{
		"auth_mode":      "apikey",
		"OPENAI_API_KEY": "sk-test",
	})

	ls := Logins("codex")
	standIn := ""
	for _, l := range ls {
		if l.Active {
			t.Fatalf("signed in with an API key, %s read active", l.User)
		}
		if l.first {
			standIn = l.User
		}
	}
	if standIn == "" {
		t.Fatalf("no stand-in among %v", ls)
	}

	p, ok := find(All(), "codex")
	if !ok || p.Account == nil {
		t.Fatal("codex left the providers with an API key in auth.json")
	}
	if !strings.EqualFold(p.Account.User, standIn) || !p.Account.standIn {
		t.Fatalf("codex listed on %q (standIn %v), want %q", p.Account.User, p.Account.standIn, standIn)
	}
	if p.Account.sign == nil || p.Account.renew == nil {
		t.Fatal("the stand-in runs on no sign-in")
	}
	if ex := savedButSignedOut(); len(ex) != 0 {
		t.Fatalf("the saved accounts are still refused: %+v", ex)
	}
}

// While Codex is signed out, the accounts ticked are the ones in use, as
// they are beside a signed-in Codex's own: ticking one in the place of
// another leaves that one out, two ticked are both used with one listed
// and the other beside it, and unticking one leaves it out.
func TestCodexStandInServesTicked(t *testing.T) {
	shellFakes(t)
	home := signIn(t)
	rememberLogins(true)
	codexSignIn(t, home, "spare@example.com", "r-spare")
	rememberLogins(true)
	writeFile(t, filepath.Join(home, ".codex", "auth.json"), map[string]any{
		"auth_mode":      "apikey",
		"OPENAI_API_KEY": "sk-test",
	})
	p, ok := find(All(), "codex")
	if !ok || p.Account == nil {
		t.Fatal("codex isn't listed")
	}
	lead, other := p.Account.User, "me@example.com"
	if strings.EqualFold(lead, other) {
		other = "spare@example.com"
	}
	// nothing ticked: Codex is listed on one saved account, none beside it
	if beside := p.AlsoOn(); len(beside) != 0 {
		t.Fatalf("an unticked account is served: %+v", beside)
	}
	if err := SetLoginOn("codex", other, true); err != nil {
		t.Fatal(err)
	}
	if p, ok = find(All(), "codex"); !ok || p.Account == nil {
		t.Fatal("codex left the providers")
	}
	if !strings.EqualFold(p.Account.User, other) {
		t.Fatalf("codex listed on %q, want the one ticked, %q", p.Account.User, other)
	}
	if beside := p.AlsoOn(); len(beside) != 0 {
		t.Fatalf("%s is served beside the stand-in", lead)
	}
	if err := SetLoginOn("codex", lead, true); err != nil {
		t.Fatal(err)
	}
	// both ticked: both in use, one listed, the other beside it
	if p, ok = find(All(), "codex"); !ok || p.Account == nil {
		t.Fatal("codex left the providers")
	}
	beside := p.AlsoOn()
	if len(beside) != 1 || beside[0].Account == nil {
		t.Fatalf("accounts beside the stand-in: %+v", beside)
	}
	listed, near := p.Account.User, beside[0].Account.User
	if strings.EqualFold(listed, near) || !sameTwo(listed, near, lead, other) {
		t.Fatalf("listed on %q beside %q, want %q and %q", listed, near, lead, other)
	}
	if beside[0].Account.sign == nil || beside[0].Account.renew == nil {
		t.Fatal("the account beside the stand-in runs on no sign-in")
	}
	if err := SetLoginOn("codex", near, false); err != nil {
		t.Fatal(err)
	}
	if p, ok = find(All(), "codex"); !ok || p.Account == nil {
		t.Fatal("codex left the providers")
	}
	if !strings.EqualFold(p.Account.User, listed) || len(p.AlsoOn()) != 0 {
		t.Fatalf("%s unticked, listed on %q with %+v beside it", near, p.Account.User, p.AlsoOn())
	}
}

// sameTwo says a and b are x and y, either way round.
func sameTwo(a, b, x, y string) bool {
	pair := func(p, q, r, s string) bool { return strings.EqualFold(p, r) && strings.EqualFold(q, s) }
	return pair(a, b, x, y) || pair(a, b, y, x)
}

// Codex signed out with Provider.NoSavedStandIn on: no saved account
// stands in, Codex leaves the list, and its accounts say they aren't
// offered again; told off, one stands in.
func TestCodexStandInOff(t *testing.T) {
	shellFakes(t)
	home := signIn(t)
	rememberLogins(true)
	codexSignIn(t, home, "spare@example.com", "r-spare")
	rememberLogins(true)
	writeFile(t, filepath.Join(home, ".codex", "auth.json"), map[string]any{
		"auth_mode":      "apikey",
		"OPENAI_API_KEY": "sk-test",
	})
	if err := SetSavedStandIn("codex", false); err != nil {
		t.Fatalf("stand-in off: %v", err)
	}
	if _, ok := find(All(), "codex"); ok {
		t.Fatal("codex listed with the stand-in off")
	}
	if ex := savedButSignedOut(); len(ex) == 0 {
		t.Fatal("its saved accounts aren't said to be not offered")
	}
	if err := SetSavedStandIn("codex", true); err != nil {
		t.Fatalf("stand-in on: %v", err)
	}
	if _, ok := find(All(), "codex"); !ok {
		t.Fatal("not listed with the stand-in on")
	}
	if ex := savedButSignedOut(); len(ex) != 0 {
		t.Fatalf("the saved accounts are still refused: %+v", ex)
	}
}

// codexStandIn's choice, the one Codex is listed on: one on before off,
// one magpie renews before one it can't, then the one seen last; lapsed
// ones left out.
func TestCodexStandInOrder(t *testing.T) {
	renew := `{"tokens":{"access_token":"a","refresh_token":"r"}}`
	at := time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC)
	l := func(user string, on, accessOnly bool, seen time.Time, auth string) savedLogin {
		return savedLogin{Agent: "codex", User: user, On: on, AccessOnly: accessOnly, Seen: seen, Auth: json.RawMessage(auth)}
	}
	lapsed := l("gone@x", true, false, at, renew)
	lapsed.Lapsed = "its sign-in is gone"
	for _, c := range []struct {
		name string
		ls   []savedLogin
		want string
	}{
		{"on before off", []savedLogin{l("off@x", false, false, at.Add(time.Hour), renew), l("on@x", true, false, at, renew)}, "on@x"},
		{"renewable before access token only", []savedLogin{l("only@x", true, true, at.Add(time.Hour), renew), l("renew@x", true, false, at, renew)}, "renew@x"},
		{"the one seen last", []savedLogin{l("old@x", false, false, at, renew), l("new@x", false, false, at.Add(time.Hour), renew)}, "new@x"},
		{"access token only, as the last one", []savedLogin{l("only@x", false, true, at, renew)}, "only@x"},
		{"lapsed passed over", []savedLogin{lapsed}, ""},
		{"nothing saved", nil, ""},
	} {
		if got := codexStandIn(c.ls); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
