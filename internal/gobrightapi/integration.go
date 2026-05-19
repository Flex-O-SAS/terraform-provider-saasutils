package gobrightapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ReadIntegration fetches an Integration by its integer Id.
// A 404 response is surfaced as an error whose message contains "not found"
// so the resource layer can detect drift via strings.Contains.
func (c *APIClient) ReadIntegration(ctx context.Context, id int64) (*Integration, error) {
	tflog.Debug(ctx, "ReadIntegration called", map[string]any{"id": id})

	path := "/api/integrations/" + strconv.FormatInt(id, 10)
	var out Integration
	status, err := c.CallInto(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, fmt.Errorf("integration %d not found", id)
		}
		return nil, err
	}
	return &out, nil
}

// readIntegrationRaw fetches the same record as ReadIntegration but as an
// opaque map. Used by Update so unmodeled fields can be carried back into
// the PUT body untouched.
func (c *APIClient) readIntegrationRaw(ctx context.Context, id int64) (IntegrationRaw, error) {
	path := "/api/integrations/" + strconv.FormatInt(id, 10)
	raw := IntegrationRaw{}
	status, err := c.CallInto(ctx, http.MethodGet, path, nil, &raw)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, fmt.Errorf("integration %d not found", id)
		}
		return nil, err
	}
	return raw, nil
}

// ListIntegrations returns the lean view (id, name, externalSystem, plus a
// few non-OIDC flags) of every integration visible to the authenticated
// principal. CreateIntegration uses it to discover the id of a freshly
// created integration, since the POST response has no body.
func (c *APIClient) ListIntegrations(ctx context.Context) ([]Integration, error) {
	tflog.Debug(ctx, "ListIntegrations called", nil)

	var out []Integration
	if _, err := c.CallInto(ctx, http.MethodGet, "/api/integrations/", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateIntegration POSTs the typed body and returns the full server-side
// representation.
//
// GoBright responds to POST /api/integrations with `201 Created` and no
// body — the new id is not exposed in the response. To recover it we list
// /api/integrations/, filter by name, and pick the entry with the greatest
// id (names are not unique on the GoBright side; the just-created record
// has the highest id by construction).
//
// Caveat: under concurrent Create operations with the same name, the
// greatest-id heuristic can pick a different actor's row. The Terraform
// resource serializes Create calls within a single plan/apply, so this is
// only an issue if another client races us.
func (c *APIClient) CreateIntegration(ctx context.Context, body *Integration) (*Integration, error) {
	tflog.Debug(ctx, "CreateIntegration called", map[string]any{"name": body.Name})

	if _, err := c.CallInto(ctx, http.MethodPost, "/api/integrations", body, nil); err != nil {
		return nil, err
	}

	list, err := c.ListIntegrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("locate newly created integration %q: %w", body.Name, err)
	}

	var newestId int64
	for _, item := range list {
		if item.Name == body.Name && item.Id > newestId {
			newestId = item.Id
		}
	}
	if newestId == 0 {
		return nil, fmt.Errorf("GoBright Create succeeded but no integration with name %q appeared in the listing", body.Name)
	}

	return c.ReadIntegration(ctx, newestId)
}

// UpdateIntegration round-trips the server's full Integration body. It
// fetches the current raw representation, overlays the user-controlled
// fields from `body`, then PUTs to /api/integrations/ (no id in path — the
// id is carried in the body). The post-PUT state is fetched via a fresh
// ReadIntegration so the caller sees the canonical server view.
//
// Fields not modeled in `Integration` (SAML, Office365, locker, sensoring,
// ...) survive the round-trip because they pass through the raw map
// untouched.
//
// Computed fields managed by the server (newId, oidcJwksEndpoint) are also
// left alone — whatever Read returned is carried back.
func (c *APIClient) UpdateIntegration(ctx context.Context, id int64, body *Integration) (*Integration, error) {
	tflog.Debug(ctx, "UpdateIntegration called", map[string]any{"id": id})

	raw, err := c.readIntegrationRaw(ctx, id)
	if err != nil {
		return nil, err
	}

	raw["id"] = id
	raw["name"] = body.Name
	raw["externalSystem"] = body.ExternalSystem
	raw["oidcAudience"] = body.OidcAudience
	raw["oidcIssuer"] = body.OidcIssuer
	raw["oidcValidationMode"] = body.OidcValidationMode
	raw["oidcPublicKey"] = body.OidcPublicKey
	raw["oidcUserIdentifierClaimName"] = body.OidcUserIdentifierClaimName
	raw["oidcRelatedUserIntegrationId"] = body.OidcRelatedUserIntegrationId
	raw["oidcTokenReplayPreventionMode"] = body.OidcTokenReplayPreventionMode

	if _, err := c.CallInto(ctx, http.MethodPut, "/api/integrations/", raw, nil); err != nil {
		return nil, err
	}

	return c.ReadIntegration(ctx, id)
}

// DeleteIntegration removes the Integration with the given Id. 404 is treated
// as success (already gone).
func (c *APIClient) DeleteIntegration(ctx context.Context, id int64) error {
	tflog.Debug(ctx, "DeleteIntegration called", map[string]any{"id": id})

	path := "/api/integrations/" + strconv.FormatInt(id, 10)
	status, err := c.CallInto(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		if status == http.StatusNotFound {
			return nil
		}
		return err
	}
	return nil
}
