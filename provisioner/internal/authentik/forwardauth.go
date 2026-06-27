package authentik

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/AndriyKalashnykov/authentik-k8s/provisioner/internal/util"
	"goauthentik.io/api/v3"
)

// Managed identifier of Authentik's built-in embedded outpost. The embedded
// outpost is the one that runs inside the authentik-server process and serves
// the `/outpost.goauthentik.io/` forward-auth endpoints — there is no separate
// container to deploy for it.
const EmbeddedOutpostManaged = "goauthentik.io/outposts/embedded"

// ForwardAuthConfig describes a forward-auth proxy provider + application pair.
type ForwardAuthConfig struct {
	// ProviderName is the display name of the proxy provider.
	ProviderName string
	// AppName / AppSlug identify the Application that surfaces the provider.
	AppName string
	AppSlug string
	// ExternalHost is the public URL Traefik fronts (e.g. https://whoami.127-0-0-1.sslip.io).
	ExternalHost string
	// Mode is the proxy mode: "forward_single" (one app — the canonical Traefik
	// demo) or "forward_domain" (domain-wide SSO). Defaults to forward_single.
	Mode string
	// CookieDomain is the domain the auth cookie is scoped to (forward_domain mode only).
	CookieDomain string
	// AuthorizationFlowSlug / InvalidationFlowSlug are resolved to PKs at runtime
	// (never transcribe flow PKs — they differ per instance).
	AuthorizationFlowSlug string
	InvalidationFlowSlug  string
}

// ListFlowsBySlug returns the flow instances matching a slug (0 or 1 in practice).
func ListFlowsBySlug(ctx context.Context, apiClient *api.APIClient, slug string) (*api.PaginatedFlowList, *http.Response, error) {
	return apiClient.FlowsAPI.FlowsInstancesList(ctx).Slug(slug).Execute()
}

// ResolveFlowPK looks up a flow's primary key by its slug. Flow PKs are
// instance-specific, so they must be resolved at runtime rather than hardcoded.
func ResolveFlowPK(ctx context.Context, apiClient *api.APIClient, slug string) (string, error) {
	list, _, err := ListFlowsBySlug(ctx, apiClient, slug)
	if err != nil {
		return "", fmt.Errorf("listing flow %q: %w", slug, err)
	}
	for _, f := range list.Results {
		if f.Slug == slug {
			return f.Pk, nil
		}
	}
	return "", fmt.Errorf("flow with slug %q not found", slug)
}

// ListProxyProviders returns all proxy providers (the list endpoint has no name filter).
func ListProxyProviders(ctx context.Context, apiClient *api.APIClient) (*api.PaginatedProxyProviderList, *http.Response, error) {
	return apiClient.ProvidersAPI.ProvidersProxyList(ctx).Execute()
}

// CreateProxyProvider creates a forwardAuth proxy provider. Mode defaults to
// forward_single; cookie_domain is only sent in forward_domain mode (it is
// meaningless for a single application).
func CreateProxyProvider(ctx context.Context, apiClient *api.APIClient, cfg ForwardAuthConfig, authzFlowPK, invalidationFlowPK string) (*api.ProxyProvider, *http.Response, error) {
	mode := api.ProxyMode(cfg.Mode)
	if cfg.Mode == "" {
		mode = api.PROXYMODE_FORWARD_SINGLE
	}
	req := api.ProxyProviderRequest{
		Name:              cfg.ProviderName,
		AuthorizationFlow: authzFlowPK,
		InvalidationFlow:  invalidationFlowPK,
		ExternalHost:      cfg.ExternalHost,
		Mode:              &mode,
	}
	if mode == api.PROXYMODE_FORWARD_DOMAIN {
		req.CookieDomain = util.StringToPointer(cfg.CookieDomain)
	}
	return apiClient.ProvidersAPI.ProvidersProxyCreate(ctx).ProxyProviderRequest(req).Execute()
}

// CreateOrGetProxyProvider returns the PK of the proxy provider named
// cfg.ProviderName, creating it if absent (idempotent).
func CreateOrGetProxyProvider(ctx context.Context, apiClient *api.APIClient, cfg ForwardAuthConfig, authzFlowPK, invalidationFlowPK string) (int32, error) {
	list, _, err := ListProxyProviders(ctx, apiClient)
	if err != nil {
		return 0, fmt.Errorf("listing proxy providers: %w", err)
	}
	for _, p := range list.Results {
		if p.Name == cfg.ProviderName {
			return p.Pk, nil
		}
	}
	created, _, err := CreateProxyProvider(ctx, apiClient, cfg, authzFlowPK, invalidationFlowPK)
	if err != nil {
		return 0, fmt.Errorf("creating proxy provider %q: %w", cfg.ProviderName, err)
	}
	return created.Pk, nil
}

// ListApplicationsBySlug returns applications matching a slug (0 or 1).
func ListApplicationsBySlug(ctx context.Context, apiClient *api.APIClient, slug string) (*api.PaginatedApplicationList, *http.Response, error) {
	return apiClient.CoreAPI.CoreApplicationsList(ctx).Slug(slug).Execute()
}

// CreateApplication creates an Application bound to a provider PK.
func CreateApplication(ctx context.Context, apiClient *api.APIClient, cfg ForwardAuthConfig, providerPK int32) (*api.Application, *http.Response, error) {
	return apiClient.CoreAPI.CoreApplicationsCreate(ctx).ApplicationRequest(api.ApplicationRequest{
		Name:     cfg.AppName,
		Slug:     cfg.AppSlug,
		Provider: *api.NewNullableInt32(util.Int32ToPointer(providerPK)),
	}).Execute()
}

// CreateOrGetApplication creates the application for cfg if its slug is not
// already present (idempotent).
func CreateOrGetApplication(ctx context.Context, apiClient *api.APIClient, cfg ForwardAuthConfig, providerPK int32) error {
	list, _, err := ListApplicationsBySlug(ctx, apiClient, cfg.AppSlug)
	if err != nil {
		return fmt.Errorf("listing applications %q: %w", cfg.AppSlug, err)
	}
	for _, a := range list.Results {
		if a.Slug == cfg.AppSlug {
			return nil
		}
	}
	if _, _, err := CreateApplication(ctx, apiClient, cfg, providerPK); err != nil {
		return fmt.Errorf("creating application %q: %w", cfg.AppSlug, err)
	}
	return nil
}

// ListOutposts returns all outpost instances.
func ListOutposts(ctx context.Context, apiClient *api.APIClient) (*api.PaginatedOutpostList, *http.Response, error) {
	return apiClient.OutpostsAPI.OutpostsInstancesList(ctx).Execute()
}

// FindEmbeddedOutpost returns the built-in embedded outpost.
func FindEmbeddedOutpost(ctx context.Context, apiClient *api.APIClient) (*api.Outpost, error) {
	list, _, err := ListOutposts(ctx, apiClient)
	if err != nil {
		return nil, fmt.Errorf("listing outposts: %w", err)
	}
	for i := range list.Results {
		if list.Results[i].Managed.Get() != nil && *list.Results[i].Managed.Get() == EmbeddedOutpostManaged {
			return &list.Results[i], nil
		}
	}
	return nil, fmt.Errorf("embedded outpost (managed %q) not found", EmbeddedOutpostManaged)
}

// BindProviderToOutpost ensures providerPK is in the outpost's provider list
// (idempotent — a no-op if already bound).
func BindProviderToOutpost(ctx context.Context, apiClient *api.APIClient, outpost *api.Outpost, providerPK int32) (*http.Response, error) {
	if slices.Contains(outpost.Providers, providerPK) {
		return nil, nil // already bound
	}
	providers := append(append([]int32{}, outpost.Providers...), providerPK)
	_, resp, err := apiClient.OutpostsAPI.OutpostsInstancesPartialUpdate(ctx, outpost.Pk).
		PatchedOutpostRequest(api.PatchedOutpostRequest{Providers: providers}).Execute()
	if err != nil {
		return resp, fmt.Errorf("binding provider %d to outpost %q: %w", providerPK, outpost.Pk, err)
	}
	return resp, nil
}

// SetupForwardAuth wires a full forward-auth demo for cfg, idempotently:
//  1. resolve the authorization + invalidation flow PKs by slug,
//  2. create (or reuse) the forward_domain proxy provider,
//  3. create (or reuse) the application bound to it,
//  4. bind the provider to the embedded outpost so the server serves its
//     /outpost.goauthentik.io/auth/* endpoints.
func SetupForwardAuth(ctx context.Context, apiClient *api.APIClient, cfg ForwardAuthConfig) error {
	authzFlowPK, err := ResolveFlowPK(ctx, apiClient, cfg.AuthorizationFlowSlug)
	if err != nil {
		return err
	}
	invalidationFlowPK, err := ResolveFlowPK(ctx, apiClient, cfg.InvalidationFlowSlug)
	if err != nil {
		return err
	}

	providerPK, err := CreateOrGetProxyProvider(ctx, apiClient, cfg, authzFlowPK, invalidationFlowPK)
	if err != nil {
		return err
	}

	if err := CreateOrGetApplication(ctx, apiClient, cfg, providerPK); err != nil {
		return err
	}

	outpost, err := FindEmbeddedOutpost(ctx, apiClient)
	if err != nil {
		return err
	}
	if _, err := BindProviderToOutpost(ctx, apiClient, outpost, providerPK); err != nil {
		return err
	}
	return nil
}
