terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
}

provider "saasutils" {
  zitadel {
    issuer  = "https://example.zitadel.cloud"
    api     = "example.zitadel.cloud:443"
    user_id = "123456789012345678"
    key     = file("${path.module}/system_user.pem")
  }
}

# Machine owner with a Personal Access Token. The PAT is returned in the
# `pat` attribute and can be used to authenticate against the new instance.
resource "saasutils_zitadel_instance" "machine_example" {
  instance_name    = "tenant-acme"
  first_org_name   = "ACME"
  custom_domain    = "acme.example.com"
  default_language = "en"

  machine {
    user_name = "automation"
    name      = "Automation"

    personal_access_token {
      expiration_date = "2027-01-01T00:00:00Z" # optional; omit for no expiration
    }
  }
}

# Human owner. Mutually exclusive with `machine`; no PAT is returned in this
# mode. Use a separate human-user resource on the new instance if you also need
# automation credentials.
resource "saasutils_zitadel_instance" "human_example" {
  instance_name    = "tenant-acme-human"
  first_org_name   = "ACME"
  custom_domain    = "acme-human.example.com"
  default_language = "en"

  human {
    user_name                = "admin"
    email                    = "admin@acme.example.com"
    is_email_verified        = true
    first_name               = "ACME"
    last_name                = "Admin"
    preferred_language       = "en"
    password                 = "ChangeMe!2025"
    password_change_required = true
  }
}

output "zitadel_instance_id" {
  value = saasutils_zitadel_instance.machine_example.id
}

output "zitadel_instance_pat" {
  value     = saasutils_zitadel_instance.machine_example.pat
  sensitive = true
}
