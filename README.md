# Contents

* [Functions](#functions)
  * [Azure ASG](#case-conversion)
    * [icase_asgid](#icase_asgid)

# Functions

## Case conversion

### icase_asgid

`icase_asgid(uppercase bool, input string) string`

Generate Azure ASG id with upper or lower case resource group and ASG name

Example:

```hcl

locals {
  input = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name"
}

output "asgid" {
  value = provider::saasutils::icase_asgid(false, local.input)
}

output "asgid-with-case" {
  value = provider::saasutils::icase_asgid(true, local.input)
}

# asgid = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name"
# asgid-with-case = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/RG-NAME/providers/Microsoft.Network/applicationSecurityGroups/ASG-NAME"
```

# Command line actions

## Generating documentation

```
go generate ./...
```

## Linting

```
golangci-lint run
```

## Running tests

```
go test -v terraform-provider-saasutils/internal/provider
```

## Running an example

```
cd examples/functions/icase_asgid
terraform init
terraform plan

Changes to Outputs:
  + asgid = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name"
  + asgid-with-case = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/RG-NAME/providers/Microsoft.Network/applicationSecurityGroups/ASG-NAME"
```

## Building for macOS (Apple Silicon)

```powershell
$env:GOOS = "darwin"; $env:GOARCH = "arm64"; go build -o terraform-provider-saasutils_darwin_arm64
```

## Building for Linux (x86_64)

```bash
GOOS=linux GOARCH=amd64 go build -o terraform-provider-saasutils_linux_amd64
```

## Building for Windows (x86_64)

```bash
GOOS=windows GOARCH=amd64 go build -o terraform-provider-saasutils_windows_amd64.exe
```
