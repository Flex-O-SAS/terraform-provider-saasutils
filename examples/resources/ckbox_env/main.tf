terraform {
  required_providers {
    saasutils = {
      source = "local/saasutils/saasutils"
      version = "0.1.0"
    }
  }
}

provider "saasutils" {
  email    = "example@example.com"
  password = "password"
}

resource "saasutils_ckbox_env" "example" {
  name = "example"
}

# data "saasutils_ckbox_env" "example" {
#   name = "test"
# }

# output "env" {
#   value = saasutils_ckbox_env.example.id
# }