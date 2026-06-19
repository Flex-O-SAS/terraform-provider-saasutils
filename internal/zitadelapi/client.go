// Package zitadelapi wraps the Zitadel Go SDK system client. It exposes the
// minimum surface the provider needs (instance lifecycle calls) and centralises
// JWT-key authentication against the Zitadel SystemAPI.
package zitadelapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/zitadel/zitadel-go/v3/pkg/client"
	systemclient "github.com/zitadel/zitadel-go/v3/pkg/client/system"
	instancepb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/instance"
	objectpb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/object/v2"
	orgpb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/org/v2"
	systempb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/system"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client is a thin wrapper around the Zitadel system gRPC client.
type Client struct {
	sys *systemclient.Client

	// insecure and apiPort describe how to reach instance-scoped APIs (e.g. the
	// org service of a freshly created instance) over the same transport as the
	// configured SystemAPI. apiPort is the port parsed from Config.API and is
	// reused for instance connections, which matters only on the insecure
	// local-dev path where all instances share one host:port.
	insecure bool
	apiPort  string
}

// Config carries the inputs needed to authenticate against the Zitadel SystemAPI.
type Config struct {
	// Issuer is the OIDC issuer of the Zitadel deployment, e.g.
	// "https://example.zitadel.cloud".
	Issuer string
	// API is the gRPC endpoint, e.g. "example.zitadel.cloud:443".
	API string
	// UserID is the SystemAPIUser id configured in Zitadel.
	UserID string
	// Key is the PEM-encoded private key matching the public key registered
	// for UserID in Zitadel's SystemAPIUsers config.
	Key []byte
	// Insecure disables TLS on the gRPC connection (local dev only).
	Insecure bool
}

// NewClient builds an authenticated Zitadel system client.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Issuer == "" || cfg.API == "" || cfg.UserID == "" || len(cfg.Key) == 0 {
		return nil, errors.New("zitadel: issuer, api, user_id and key are all required")
	}

	var opts []systemclient.Option
	if cfg.Insecure {
		opts = append(opts, systemclient.WithInsecure())
	}

	sys, err := systemclient.NewClient(
		ctx,
		cfg.Issuer,
		cfg.API,
		systemclient.JWTProfileFromKey(cfg.Key, cfg.UserID),
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("zitadel: connect: %w", err)
	}

	// Best-effort extract the port from the configured gRPC endpoint so that
	// instance-scoped connections can reuse it. Defaults to 443 (TLS).
	apiPort := "443"
	if _, port, splitErr := net.SplitHostPort(cfg.API); splitErr == nil && port != "" {
		apiPort = port
	}

	return &Client{sys: sys, insecure: cfg.Insecure, apiPort: apiPort}, nil
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	if c == nil || c.sys == nil || c.sys.Connection == nil {
		return nil
	}
	return c.sys.Connection.Close()
}

// CreateInstance creates a new Zitadel instance and returns the response,
// which contains the new instance id and bootstrap credentials.
func (c *Client) CreateInstance(ctx context.Context, req *systempb.CreateInstanceRequest) (*systempb.CreateInstanceResponse, error) {
	return c.sys.CreateInstance(ctx, req)
}

// GetInstance fetches an instance by id. Returns (nil, nil) when the instance
// is gone, so the caller can drop it from state.
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*instancepb.InstanceDetail, error) {
	// SystemAPI GetInstance is deprecated upstream in favour of the v2 instance
	// service, but that operates only on the instance in the request context;
	// the system client needs to address instances by id, so this stays.
	resp, err := c.sys.GetInstance(ctx, &systempb.GetInstanceRequest{InstanceId: instanceID}) //nolint:staticcheck
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	return resp.GetInstance(), nil
}

// UpdateInstanceName renames an existing instance.
func (c *Client) UpdateInstanceName(ctx context.Context, instanceID, name string) error {
	// See GetInstance: the SystemAPI variant is needed to target by instance id.
	_, err := c.sys.UpdateInstance(ctx, &systempb.UpdateInstanceRequest{ //nolint:staticcheck
		InstanceId:   instanceID,
		InstanceName: name,
	})
	return err
}

// RemoveInstance deletes an instance. A NotFound error is treated as success.
func (c *Client) RemoveInstance(ctx context.Context, instanceID string) error {
	// See GetInstance: the SystemAPI variant is needed to target by instance id.
	_, err := c.sys.RemoveInstance(ctx, &systempb.RemoveInstanceRequest{InstanceId: instanceID}) //nolint:staticcheck
	if err != nil && status.Code(err) != codes.NotFound {
		return err
	}
	return nil
}

// WaitForInstanceRunning polls GetInstance until the instance reports
// State_STATE_RUNNING or the timeout elapses, returning the running
// InstanceDetail. On timeout it returns an error naming the last observed
// state. A not-yet-visible instance (NotFound right after creation) is treated
// as "still starting" and keeps the poll alive.
func (c *Client) WaitForInstanceRunning(ctx context.Context, instanceID string, timeout time.Duration) (*instancepb.InstanceDetail, error) {
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	last := instancepb.State_STATE_UNSPECIFIED
	for {
		detail, err := c.GetInstance(pollCtx, instanceID)
		switch {
		case err != nil && pollCtx.Err() == nil:
			// A genuine API error (not the poll timeout) is fatal.
			return nil, err
		case err == nil && detail != nil:
			if last = detail.GetState(); last == instancepb.State_STATE_RUNNING {
				return detail, nil
			}
		}

		select {
		case <-pollCtx.Done():
			return nil, fmt.Errorf("timed out after %s waiting for instance %q to reach STATE_RUNNING (last observed state: %s)", timeout, instanceID, last)
		case <-ticker.C:
		}
	}
}

// OrgLookup carries the inputs needed to find an organization on a freshly
// created instance. Domain is the instance host to dial; OrgName is the
// organization to match by exact name. Credentials are either a machine user's
// PAT, or a human owner's Username/Password.
type OrgLookup struct {
	Domain   string
	OrgName  string
	PAT      string
	Username string
	Password string
}

// FindOrganizationID connects to the instance at l.Domain with the supplied
// credentials and returns the id of the organization named l.OrgName.
func (c *Client) FindOrganizationID(ctx context.Context, l OrgLookup) (string, error) {
	var auth client.Option
	switch {
	case l.PAT != "":
		auth = client.WithAuth(client.PAT(l.PAT))
	case l.Username != "" && l.Password != "":
		auth = client.WithAuth(client.PasswordAuthentication(l.Username, l.Password, client.ScopeZitadelAPI()))
	default:
		return "", errors.New("no credentials available to query organizations (need a PAT or human owner credentials)")
	}

	zOpts := []zitadel.Option{}
	if c.insecure {
		zOpts = append(zOpts, zitadel.WithInsecure(c.apiPort))
	}

	api, err := client.New(ctx, zitadel.New(l.Domain, zOpts...), auth)
	if err != nil {
		return "", fmt.Errorf("connect to instance %q: %w", l.Domain, err)
	}
	defer api.Close()

	resp, err := api.OrganizationServiceV2().ListOrganizations(ctx, &orgpb.ListOrganizationsRequest{
		Queries: []*orgpb.SearchQuery{{
			Query: &orgpb.SearchQuery_NameQuery{
				NameQuery: &orgpb.OrganizationNameQuery{
					Name:   l.OrgName,
					Method: objectpb.TextQueryMethod_TEXT_QUERY_METHOD_EQUALS,
				},
			},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("list organizations on %q: %w", l.Domain, err)
	}

	for _, org := range resp.GetResult() {
		if org.GetName() == l.OrgName {
			return org.GetId(), nil
		}
	}
	return "", fmt.Errorf("organization %q not found on instance %q", l.OrgName, l.Domain)
}
