package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeServiceAccount(t *testing.T, tokenURL string) string {
	t.Helper()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	b, _ := json.Marshal(map[string]string{"client_email": "sa@proj.iam.gserviceaccount.com", "private_key": pemStr, "token_uri": tokenURL})
	return string(b)
}

func TestPlayTokenExchangesSignedJWT(t *testing.T) {
	var assertion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		assertion = r.PostForm.Get("assertion")
		if r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type = %s", r.PostForm.Get("grant_type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"ya29.x","expires_in":3600}`))
	}))
	defer srv.Close()
	tok, err := playToken(fakeServiceAccount(t, srv.URL), time.Unix(1700000000, 0), "")
	if err != nil || tok != "ya29.x" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	parts := strings.Split(assertion, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(payload, &claims)
	if claims["iss"] != "sa@proj.iam.gserviceaccount.com" || claims["scope"] != "https://www.googleapis.com/auth/androidpublisher" || claims["aud"] != srv.URL {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestPlayDecision(t *testing.T) {
	if err := playDecision(playProduct{OrderID: "GPA.1", PurchaseState: 0}); err != nil {
		t.Fatalf("已购买应通过: %v", err)
	}
	if err := playDecision(playProduct{OrderID: "GPA.1", PurchaseState: 1}); err == nil {
		t.Fatal("已取消/退款不该入账")
	}
	if err := playDecision(playProduct{OrderID: "GPA.1", PurchaseState: 2}); err == nil {
		t.Fatal("待处理不该入账")
	}
	if err := playDecision(playProduct{PurchaseState: 0}); err == nil {
		t.Fatal("没有 orderId 不该入账(没法幂等)")
	}
}

func TestPlayProductParse(t *testing.T) {
	raw := `{"purchaseTimeMillis":"1700000000000","purchaseState":0,"consumptionState":0,"orderId":"GPA.3333-1111","acknowledgementState":0,"regionCode":"IN","kind":"androidpublisher#productPurchase"}`
	var p playProduct
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	if p.OrderID != "GPA.3333-1111" || p.RegionCode != "IN" || p.PurchaseState != 0 {
		t.Fatalf("%+v", p)
	}
}
