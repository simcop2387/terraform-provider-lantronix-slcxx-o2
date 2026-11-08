// Package provider exposes the lantronix_slcxx_o2 Terraform provider.
package provider

import (
	"context"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/slc"
	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/transport"
)

const (
	envUsername = "LANTRONIX_SLCXX_O2_USERNAME"
	envPassword = "LANTRONIX_SLCXX_O2_PASSWORD"
)

var _ provider.Provider = (*slcProvider)(nil)

type slcProvider struct{ version string }

// New returns the provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &slcProvider{version: version} }
}

type providerModel struct {
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	LoginRetries   types.Int64  `tfsdk:"login_retries"`
	CommandTimeout types.Int64  `tfsdk:"command_timeout"`
}

func (p *slcProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "lantronix_slcxx_o2"
	resp.Version = p.version
}

func (p *slcProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Lantronix SecureLinx SLCxx-O2 console servers over their CLI (telnet transport).",
		Attributes: map[string]schema.Attribute{
			"host":            schema.StringAttribute{Required: true, Description: "Hostname or IP of the SLC management interface."},
			"port":            schema.Int64Attribute{Optional: true, Description: "Telnet port. Defaults to 23."},
			"username":        schema.StringAttribute{Optional: true, Sensitive: true, Description: "CLI user (full-CLI admin required). Defaults to sysadmin or `" + envUsername + "`."},
			"password":        schema.StringAttribute{Optional: true, Sensitive: true, Description: "CLI password. Falls back to `" + envPassword + "`."},
			"login_retries":   schema.Int64Attribute{Optional: true, Description: "Extra full-login attempts after a rejection (device lockout guard). Default 1."},
			"command_timeout": schema.Int64Attribute{Optional: true, Description: "Per-command timeout in seconds. Default 60 (network-affecting commands can take 60-120s)."},
		},
	}
}

func (p *slcProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.Host.IsNull() || cfg.Host.IsUnknown() || cfg.Host.ValueString() == "" {
		resp.Diagnostics.AddError("Missing host", `The provider requires "host".`)
		return
	}
	out := transport.Config{Host: cfg.Host.ValueString()}
	if !cfg.Port.IsNull() && !cfg.Port.IsUnknown() {
		out.Port = int(cfg.Port.ValueInt64())
	}
	switch u, e := os.LookupEnv(envUsername); {
	case !cfg.Username.IsNull() && !cfg.Username.IsUnknown():
		out.Username = cfg.Username.ValueString()
	case e:
		out.Username = u
	}
	switch pw, e := os.LookupEnv(envPassword); {
	case !cfg.Password.IsNull() && !cfg.Password.IsUnknown():
		out.Password = cfg.Password.ValueString()
	case e:
		out.Password = pw
	}
	if out.Password == "" {
		resp.Diagnostics.AddError("Missing password", `Set "password" or `+envPassword+".")
		return
	}
	if !cfg.LoginRetries.IsNull() && !cfg.LoginRetries.IsUnknown() {
		out.LoginRetries = int(cfg.LoginRetries.ValueInt64())
	}
	if !cfg.CommandTimeout.IsNull() && !cfg.CommandTimeout.IsUnknown() {
		out.CommandTimeout = time.Duration(cfg.CommandTimeout.ValueInt64()) * time.Second
	}

	client := slc.NewClient(transport.Dial(out))
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *slcProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewPortResource, NewPortAccessResource}
}

func (p *slcProvider) DataSources(_ context.Context) []func() datasource.DataSource { return nil }
