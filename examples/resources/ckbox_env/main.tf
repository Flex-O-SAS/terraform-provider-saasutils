terraform {
  required_providers {
    saasutils = {
      source = "local/saasutils/saasutils"
      version = "0.1.0"
    }
  }
}

provider "saasutils" {
  email    = "platform.team@saas-office.com"
  password = "P8T-oUtKTJr8-v8ZPZELD999yrZ3uX39sPtMJVcudp"
}

resource "saasutils_ckbox_env" "example" {
  name = "test"
}

resource "saasutils_ckbox_access_key" "example" {
  env_id  = saasutils_ckbox_env.example
  name    = "Salut"
}


# data "saasutils_ckbox_env" "example" {
#   name = "test"
# }

# output "env" {
#   value = saasutils_ckbox_env.example.id
# }