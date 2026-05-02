package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestAlertsHandler_JoinsProvider(t *testing.T) {
	objs := []client.Object{
		mkAlert("prod-pagerduty", "flux-system", "pagerduty-prov", "error", false),
		mkAlert("shop-slack", "flux-system", "slack-prov", "info", false),
		mkAlert("paused-route", "flux-system", "slack-prov", "info", true),
		mkAlert("orphan", "flux-system", "missing-provider", "info", false),
		mkProvider("pagerduty-prov", "flux-system", "pagerduty", "", "https://events.pagerduty.com/v2/enqueue"),
		mkProvider("slack-prov", "flux-system", "slack", "#shop-deploys", ""),
	}
	e := newAlertsEntry("alpha", "Alpha", objs...)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &alertsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp types.AlertsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Alerts) != 4 {
		t.Fatalf("got %d alerts, want 4", len(resp.Alerts))
	}

	by := map[string]types.Alert{}
	for _, a := range resp.Alerts {
		by[a.Name] = a
	}
	if got := by["prod-pagerduty"]; got.Provider != "pagerduty" || got.Target == "" {
		t.Errorf("prod-pagerduty resolved wrong: %+v", got)
	}
	if got := by["shop-slack"]; got.Provider != "slack" || got.Target != "#shop-deploys" {
		t.Errorf("shop-slack resolved wrong: %+v", got)
	}
	if got := by["paused-route"]; got.Status != "paused" || !got.Suspended {
		t.Errorf("paused-route should be paused: %+v", got)
	}
	if got := by["orphan"]; got.Provider != "missing" {
		t.Errorf("orphan should resolve to missing provider: %+v", got)
	}
}

// ---------- fixtures ----------

func newAlertsEntry(id, displayName string, objs ...client.Object) *cluster.Entry {
	builder := fake.NewClientBuilder().WithScheme(cluster.RemoteScheme())
	if len(objs) > 0 {
		builder = builder.WithObjects(objs...)
	}
	e := &cluster.Entry{
		Name:          id,
		DisplayName:   displayName,
		FluxNamespace: "flux-system",
		Client:        builder.Build(),
		Discovery:     &fakeDiscovery{},
	}
	e.SetStatus(cluster.Status{Reachable: true, FluxInstalled: true})
	return e
}

func mkAlert(name, ns, providerName, severity string, suspend bool) *notificationv1beta3.Alert {
	return &notificationv1beta3.Alert{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: notificationv1beta3.AlertSpec{
			ProviderRef:   fluxmeta.LocalObjectReference{Name: providerName},
			EventSeverity: severity,
			Suspend:       suspend,
		},
	}
}

func mkProvider(name, ns, ptype, channel, address string) *notificationv1beta3.Provider {
	return &notificationv1beta3.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: notificationv1beta3.ProviderSpec{
			Type:    ptype,
			Channel: channel,
			Address: address,
		},
	}
}
