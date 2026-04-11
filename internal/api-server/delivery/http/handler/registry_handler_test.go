package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/idempotency"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/registry"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"

	"github.com/gofiber/fiber/v3"
)

type snapRepoStub struct {
	keys    []string
	byKey   map[string][]sharedgit.ContractSnapshot
	listErr error
	getErr  error
}

func (s *snapRepoStub) ListBundleKeys(context.Context) ([]string, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.keys, nil
}

func (s *snapRepoStub) GetSnapshots(_ context.Context, bk string) ([]sharedgit.ContractSnapshot, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.byKey == nil {
		return nil, nil
	}
	return s.byKey[bk], nil
}

func (s *snapRepoStub) SaveSnapshots(context.Context, string, []sharedgit.ContractSnapshot) (bool, error) {
	panic("unexpected in handler test")
}

type ctlRepoStub struct {
	list     []models.ControllerInfo
	err      error
	get      *models.ControllerInfo
	getErr   error
	hbeat    time.Time
	hbeatErr error
}

func (c *ctlRepoStub) RegisterController(context.Context, models.ControllerInfo) error {
	panic("unexpected")
}

func (c *ctlRepoStub) GetController(context.Context, string) (*models.ControllerInfo, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.get, nil
}

func (c *ctlRepoStub) GetHeartbeat(context.Context, string) (time.Time, error) {
	if c.hbeatErr != nil {
		return time.Time{}, c.hbeatErr
	}
	return c.hbeat, nil
}

func (c *ctlRepoStub) ListControllers(context.Context) ([]models.ControllerInfo, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.list, nil
}

func (c *ctlRepoStub) UpdateControllerHeartbeat(context.Context, string, []models.EnvironmentInfo) (bool, error) {
	panic("unexpected")
}

type bundleSyncStub struct {
	snaps []sharedgit.ContractSnapshot
	err   error
}

func (b *bundleSyncStub) SyncBundle(context.Context, models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	return b.snaps, b.err
}

func (b *bundleSyncStub) StartBundleWatcher(context.Context) {}

type idemAlwaysConflict struct{}

func (idemAlwaysConflict) Execute(context.Context, string, string, func() (*idempotency.HTTPResult, error)) (*idempotency.HTTPResult, error) {
	return nil, idempotency.ErrConflict
}

// pingOK satisfies interfaces.ContractSyncerReachability for StatusReadUseCase.CheckContractSyncer.
type pingOK struct{}

func (pingOK) Ping(context.Context) error { return nil }

func testSnap() sharedgit.ContractSnapshot {
	return sharedgit.ContractSnapshot{
		Name:                  "api",
		Prefix:                "/p",
		Upstream:              sharedgit.ContractUpstream{Name: "u"},
		AllowUndefinedMethods: false,
		Access:                sharedgit.Access{},
	}
}

func TestRegistryHandler_ListBundleKeys_OK(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{keys: []string{"a", "b"}}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return h.ListBundleKeys(c, apiserver.ListBundleKeysParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body apiserver.BundleKeyListResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items %#v", body.Items)
	}
}

func TestRegistryHandler_ListBundleKeys_respondError(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{listErr: apierrors.ErrStoreAccess}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return h.ListBundleKeys(c, apiserver.ListBundleKeysParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRegistryHandler_SyncBundle_validation(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{snaps: []sharedgit.ContractSnapshot{testSnap()}}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		return h.SyncBundle(c, apiserver.SyncBundleParams{})
	})

	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(`not json`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("bad json: status %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(`{"repository":"","ref":"r","bundle":"b"}`))
	req2.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("empty repo: status %d", resp2.StatusCode)
	}
}

func TestRegistryHandler_SyncBundle_success(t *testing.T) {
	t.Parallel()
	repo, ref, bname := "org/r", "main", "pkg"
	bk := bundlekey.Build(repo, ref, "")
	snap := &snapRepoStub{byKey: map[string][]sharedgit.ContractSnapshot{bk: {}}}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{snaps: []sharedgit.ContractSnapshot{testSnap()}}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		return h.SyncBundle(c, apiserver.SyncBundleParams{})
	})
	body := `{"repository":"` + repo + `","ref":"` + ref + `","bundle":"` + bname + `"}`
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestRegistryHandler_SyncBundle_idempotencyConflict(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		idemAlwaysConflict{},
	)
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		key := "0123456789abcdef0123456789abcdef"
		return h.SyncBundle(c, apiserver.SyncBundleParams{IdempotencyKey: &key})
	})
	body := `{"repository":"r","ref":"main","bundle":"b"}`
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRegistryHandler_GetReady_notReady(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/ready", func(c fiber.Ctx) error {
		return h.GetReady(c)
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRegistryHandler_GetController_notFound(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{getErr: apierrors.ErrNotFound}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/c/:id", func(c fiber.Ctx) error {
		return h.GetController(c, "missing", apiserver.GetControllerParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/c/missing", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRegistryHandler_GetControllerHeartbeat_notFound(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{hbeatErr: apierrors.ErrNotFound}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/hb/:id", func(c fiber.Ctx) error {
		return h.GetControllerHeartbeat(c, "x", apiserver.GetControllerHeartbeatParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/hb/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRegistryHandler_ListControllers_JSON(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{
			list: []models.ControllerInfo{{ControllerID: "c1", Tenant: "t1"}},
		}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return h.ListControllers(c, apiserver.ListControllersParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body apiserver.ControllerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].ControllerId != "c1" {
		t.Fatalf("items %#v", body.Items)
	}
}

func TestRegistryHandler_GetContractInBundle_notFound(t *testing.T) {
	t.Parallel()
	bk := bundlekey.Build("r", "m", "")
	snap := &snapRepoStub{byKey: map[string][]sharedgit.ContractSnapshot{bk: {}}}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/doc", func(c fiber.Ctx) error {
		return h.GetContractInBundle(c, apiserver.BundleKey(bk), "missing", apiserver.GetContractInBundleParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/doc", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSnapshotsToAPI_roundtrip(t *testing.T) {
	t.Parallel()
	in := []sharedgit.ContractSnapshot{testSnap()}
	out, err := snapshotsToAPI(in)
	if err != nil || len(out) != 1 || out[0].Name == nil || *out[0].Name != "api" {
		t.Fatalf("err=%v out=%#v", err, out)
	}
}

func TestControllerToAPI_bundleToAPI_environmentToAPI(t *testing.T) {
	t.Parallel()
	c := models.ControllerInfo{
		ControllerID: "id",
		Tenant:       "ten",
		Environments: []models.EnvironmentInfo{
			{
				Name: "e1",
				Bundles: []models.BundleInfo{
					{Name: "b1", Repository: "r", Ref: "ref", Path: "p"},
				},
			},
		},
	}
	ac := controllerToAPI(c)
	if ac.ControllerId != "id" || ac.Tenant != "ten" || ac.Environments == nil {
		t.Fatal()
	}
	ev := environmentToAPI(c.Environments[0])
	if ev.Name != "e1" || ev.Bundles == nil {
		t.Fatal()
	}
	b := bundleToAPI(models.BundleInfo{Name: "n", Repository: "r", Ref: "x", Path: "y"})
	if b.Name == nil || *b.Name != "n" {
		t.Fatal()
	}
}

func TestRegistryHandler_GetStatus_OK(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(&ctlRepoStub{}),
		registry.NewTenantReadUseCase(&ctlRepoStub{}),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, pingOK{}),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/s", func(c fiber.Ctx) error {
		return h.GetStatus(c, apiserver.GetStatusParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/s", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRegistryHandler_ListTenants_OK(t *testing.T) {
	t.Parallel()
	snap := &snapRepoStub{}
	repo := &ctlRepoStub{list: []models.ControllerInfo{{Tenant: "a"}, {Tenant: "b"}}}
	h := NewRegistryHandler(
		bundle.NewBundleReadUseCase(snap),
		registry.NewControllerReadUseCase(repo),
		registry.NewTenantReadUseCase(repo),
		bundle.NewBundleHTTPSyncUseCase(snap, &bundleSyncStub{}),
		registry.NewStatusReadUseCase(nil, nil),
		false,
		nil,
	)
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return h.ListTenants(c, apiserver.ListTenantsParams{})
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
