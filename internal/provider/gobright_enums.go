package provider

import "fmt"

// GoBright API integer enums are surfaced through the Terraform schema as
// strings or booleans for ergonomic HCL. The helpers in this file translate
// between the two representations; the gobrightapi package itself stays
// integer-only.

// externalSystem encodes the integration type the user wants.
const (
	externalSystemOffice365 int64 = 1
	externalSystemOpenID    int64 = 14
)

// supportedExternalSystemNames is the canonical list of accepted values for
// the `external_system` attribute. Order matters for the `stringvalidator.OneOf`
// diagnostic — keep the most common case first.
func supportedExternalSystemNames() []string {
	return []string{"openid", "office365"}
}

// externalSystemFromString maps an `external_system` attribute value to the
// integer GoBright expects on the wire.
func externalSystemFromString(name string) (int64, error) {
	switch name {
	case "office365":
		return externalSystemOffice365, nil
	case "openid":
		return externalSystemOpenID, nil
	}
	return 0, fmt.Errorf("unsupported external_system %q (expected one of: %v)", name, supportedExternalSystemNames())
}

// externalSystemToString maps a GoBright `externalSystem` integer back to the
// attribute value the user sees. An unrecognized integer is an error because
// the provider does not model other integration types.
func externalSystemToString(v int64) (string, error) {
	switch v {
	case externalSystemOffice365:
		return "office365", nil
	case externalSystemOpenID:
		return "openid", nil
	}
	return "", fmt.Errorf("unrecognized GoBright externalSystem=%d; this provider only manages %v", v, supportedExternalSystemNames())
}

// intToBool is the bool projection of GoBright's 0/1 integer fields
// (oidcTokenReplayPreventionMode, microsoftPermissionMode, ...). Any non-zero
// value maps to true so we don't surprise the user if the API ever returns
// 2 or similar.
func intToBool(v int64) bool {
	return v != 0
}

// boolToInt is the inverse used when building API request bodies.
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
