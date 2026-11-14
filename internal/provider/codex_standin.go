package provider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/catalog"
)

// codexStandIn is the saved ChatGPT account served in the place of Codex's
// own while nothing is signed in in ~/.codex (an API key there, or no
// auth.json): an account that can be used, one on before off, and one
// magpie can renew before one it can't (access token only), then the one
// seen last; "" when there is none. One lapsed is passed over. Ticking an
// account is what puts it in use beside a signed-in Codex's own, and it
// does the same here: the account ticked in the place of another leaves
// that one out, and the others ticked are served beside it
// (Provider.AlsoOn).
func codexStandIn(ls []savedLogin) string {
	if !SavedStandIn() {
		return ""
	}
	user, best := "", -1
	var seen time.Time
	for _, l := range ls {
		if l.Agent != "codex" || l.Lapsed != "" || !codexSavedUsable(l) {
			continue
		}
		rank := 0
		if l.On {
			rank += 2
		}
		if codexSavedRefreshable(l) {
			rank++
		}
		if rank > best || rank == best && l.Seen.After(seen) {
			user, seen, best = l.User, l.Seen, rank
		}
	}
	return user
}

// codexSavedUsable says l's saved sign-in holds an access token magpie can
// send: one it renews, or one kept access token only.
func codexSavedUsable(l savedLogin) bool {
	var a codexAuth
	return json.Unmarshal(l.Auth, &a) == nil && a.Tokens.AccessToken != ""
}

// codexSavedRefreshable says l's saved sign-in holds a refresh token
// magpie can renew it with: one kept access token only (AccessOnly) may
// hold one, and is never renewed with it, so it stops at its expiry.
func codexSavedRefreshable(l savedLogin) bool {
	var a codexAuth
	return !l.AccessOnly && json.Unmarshal(l.Auth, &a) == nil && a.Tokens.RefreshToken != ""
}

// codexSignedOut says nothing is signed in in Codex's folder under home, as
// far as magpie can see: signed out there, or an API key where a sign-in
// goes — what codexAccount takes as codexStandInAccount's. A store magpie
// can't read is unknown, never signed out.
func codexSignedOut(home string) bool {
	b, _, err := readCodexLogin(home, false)
	if codexUnread(err) {
		return false
	}
	var a codexAuth
	return err != nil || json.Unmarshal(b, &a) != nil || a.Tokens.AccessToken == "" || a.AuthMode == "apikey"
}

// codexStandInAccount is the Codex provider while nothing is signed in in
// ~/.codex, on codexStandIn's account: its sign-in runs the requests and
// its plan the models, as a signed-in Codex's own does, with the saved
// accounts ticked served beside it (Provider.AlsoOn); false with no such
// account saved.
func codexStandInAccount() (Provider, bool) {
	loginsMu.Lock()
	ls := readLogins()
	loginsMu.Unlock()
	user := codexStandIn(ls)
	if user == "" {
		return Provider{}, false
	}
	acct := &Account{Agent: "codex", Stream: true, User: user, standIn: true}
	for _, l := range ls {
		if l.Agent == "codex" && strings.EqualFold(l.User, user) {
			acct.Plan = l.Plan
		}
	}
	acct.sign = codexSign(func(ctx context.Context) (string, string, error) {
		return savedLoginToken(ctx, "codex", user)
	})
	acct.renew = func(ctx context.Context, stale string) (string, string, error) {
		return renewSavedLoginFrom(ctx, "codex", user, stale)
	}
	acct.body = codexBody
	acct.models = catalog.Codex
	acct.listedOnly = true
	acct.fetch = func(ctx context.Context) ([]catalog.Model, error) {
		ms, ver, err := codexModelsAsked(ctx, acct.sign)
		if err != nil {
			return nil, err
		}
		catalog.SaveLiveAsked(accountModels("codex", acct.User), CodexBase, ver, ms)
		codexFetchSaved(ctx)
		ms = codexPoolModels(ms)
		return ms, catalog.SaveLive("codex", CodexBase, ms)
	}
	return Provider{ID: "codex", Name: "Codex", Icon: "codex-color", Responses: CodexBase, Website: "https://chatgpt.com/codex", Account: acct}, true
}
