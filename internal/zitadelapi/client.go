// Package zitadelapi wraps the Zitadel Go SDK system client. It exposes the
// minimum surface the provider needs (instance lifecycle calls) and centralises
// JWT-key authentication against the Zitadel SystemAPI.
package zitadelapi

import (
	"context"
	"errors"
	"fmt"

	systemclient "github.com/zitadel/zitadel-go/v3/pkg/client/system"
	instancepb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/instance"
	systempb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/system"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client is a thin wrapper around the Zitadel system gRPC client.
type Client struct {
	sys *systemclient.Client
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
	return &Client{sys: sys}, nil
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
