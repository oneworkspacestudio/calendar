package testidp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/go-jose/go-jose/v4"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type Provider struct {
	Server  *httptest.Server
	mu      sync.Mutex
	codes   map[string]url.Values
	signer  jose.Signer
	Subject string
}

func New(t *testing.T) *Provider {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
	p := &Provider{codes: map[string]url.Values{}, signer: signer, Subject: "11111111-1111-4111-8111-111111111111"}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": p.Server.URL, "authorization_endpoint": p.Server.URL + "/authorize", "token_endpoint": p.Server.URL + "/token", "userinfo_endpoint": p.Server.URL + "/userinfo", "jwks_uri": p.Server.URL + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
		case "/authorize":
			params := r.URL.Query()
			code := params.Get("state")
			p.mu.Lock()
			p.codes[code] = params
			p.mu.Unlock()
			callback, _ := url.Parse(params.Get("redirect_uri"))
			query := callback.Query()
			query.Set("code", code)
			query.Set("state", code)
			callback.RawQuery = query.Encode()
			http.Redirect(w, r, callback.String(), 302)
		case "/token":
			r.ParseForm()
			p.mu.Lock()
			params, ok := p.codes[r.Form.Get("code")]
			delete(p.codes, r.Form.Get("code"))
			p.mu.Unlock()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !ok || params.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) {
				http.Error(w, "PKCE rejected", 400)
				return
			}
			claims := map[string]any{"iss": p.Server.URL, "sub": p.Subject, "aud": "shared-test", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": params.Get("nonce"), "name": "Test Owner", "preferred_username": "test-owner", "email": "owner@example.test"}
			raw, _ := json.Marshal(claims)
			signed, _ := p.signer.Sign(raw)
			token, _ := signed.CompactSerialize()
			json.NewEncoder(w).Encode(map[string]any{"access_token": token, "id_token": token, "token_type": "Bearer", "expires_in": 3600})
		case "/userinfo":
			parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
			if len(parts) != 3 {
				http.Error(w, "Unauthorized", 401)
				return
			}
			raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
			w.Write(raw)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(p.Server.Close)
	return p
}
