package main

import (
	"context"
	"flag"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/provider"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with support for debuggers (dlv attach)")
	flag.Parse()

	providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/simcop2387/lantronix_slcxx_o2",
		Debug:   debug,
	})
}
