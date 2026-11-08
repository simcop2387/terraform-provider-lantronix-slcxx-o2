terraform {
  required_providers {
    lantronix-slcxx-o2 = {
      source = "simcop2387/lantronix-slcxx-o2"
    }
  }
}

provider "lantronix-slcxx-o2" {
  host     = "192.168.0.197"
  username = "sysadmin"
  # password via LANTRONIX_SLCXX_O2_PASSWORD
}

# Smoke config: no managed resources, exercises provider schema + plan path.
