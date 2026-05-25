package subscription

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omo/internal/auth"
	"omo/internal/store"
)

func TestPublicContentIncludesActiveServiceMetadata(t *testing.T) {
	ctx := context.Background()
	appStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "omo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer appStore.Close()
	if err := appStore.EnsureServiceProfile(ctx, "standard-secure-access", "2026.05.1", "Standard secure access", "sing-box tls/tcp"); err != nil {
		t.Fatalf("ensure service profile: %v", err)
	}
	if _, err := appStore.CreateServiceInstance(ctx, "standard-secure-access", "Team access", 21080, "active", "cfg001"); err != nil {
		t.Fatalf("create active service: %v", err)
	}
	token := "subscription-token"
	if _, err := appStore.CreateDistributionToken(ctx, "Operations devices", auth.HashToken(token), nil); err != nil {
		t.Fatalf("create distribution token: %v", err)
	}

	response, err := NewService(appStore, "https://ops.example.com").PublicContent(ctx, PublicRequest{
		Token:      token,
		Format:     "sing-box",
		ClientHint: "sing-box",
		RemoteAddr: "127.0.0.1:12345",
		BaseURL:    "https://ops.example.com",
	})
	if err != nil {
		t.Fatalf("public content: %v", err)
	}
	body := string(response.Body)
	if !strings.Contains(body, `"services"`) || !strings.Contains(body, `"Team access"`) || !strings.Contains(body, `"listenPort": 21080`) || !strings.Contains(body, `"type": "trojan"`) || !strings.Contains(body, `"transport"`) {
		t.Fatalf("expected active service metadata in subscription, got %s", body)
	}
	clash, err := NewService(appStore, "https://ops.example.com").PublicContent(ctx, PublicRequest{
		Token:      token,
		Format:     "clash",
		ClientHint: "clash",
		RemoteAddr: "127.0.0.1:12345",
		BaseURL:    "https://ops.example.com",
	})
	if err != nil {
		t.Fatalf("clash public content: %v", err)
	}
	clashBody := string(clash.Body)
	if !strings.Contains(clashBody, "type: trojan") || strings.Contains(clashBody, "proxies: []") || !strings.Contains(clashBody, "network: ws") || strings.Contains(clashBody, `MATCH,"Operations devices"`) {
		t.Fatalf("expected concrete Clash proxy entry, got %s", clashBody)
	}
	uri, err := NewService(appStore, "https://ops.example.com").PublicContent(ctx, PublicRequest{
		Token:      token,
		Format:     "uri",
		ClientHint: "shadowrocket",
		RemoteAddr: "127.0.0.1:12345",
		BaseURL:    "https://ops.example.com",
	})
	if err != nil {
		t.Fatalf("uri public content: %v", err)
	}
	uriBody := string(uri.Body)
	if !strings.Contains(uriBody, "trojan://") || strings.Contains(uriBody, "https://ops.example.com/s/") {
		t.Fatalf("expected concrete client URI, got %s", uriBody)
	}

	expired := time.Now().UTC().Add(-time.Minute)
	if _, err := appStore.CreateDistributionToken(ctx, "Expired", auth.HashToken("expired-token"), &expired); err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := NewService(appStore, "https://ops.example.com").PublicContent(ctx, PublicRequest{Token: "expired-token"}); err != ErrSubscriptionNotFound {
		t.Fatalf("expected expired token rejection, got %v", err)
	}
}

func TestQRCodeSVGSupportsLongPublicImportURL(t *testing.T) {
	payload := "https://panel-with-a-long-operations-domain.example.com/s/abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789?from=qr"
	body, err := qrSVG(payload)
	if err != nil {
		t.Fatalf("qr svg: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "<svg") || !strings.Contains(text, `<path fill="#000"`) {
		t.Fatalf("expected scannable svg output, got %s", text)
	}
}
