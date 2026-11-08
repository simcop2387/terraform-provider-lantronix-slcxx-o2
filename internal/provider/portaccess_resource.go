package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/slc"
)

var (
	_ resource.Resource                = (*portAccessResource)(nil)
	_ resource.ResourceWithImportState = (*portAccessResource)(nil)
)

// NewPortAccessResource manages network access of one Device Port. Destroying
// it disables all three services (#9).
func NewPortAccessResource() resource.Resource { return &portAccessResource{} }

type portAccessResource struct{ client *slc.Client }

type portAccessModel struct {
	Port          types.Int64 `tfsdk:"port"`
	TelnetEnabled types.Bool  `tfsdk:"telnet_enabled"`
	TelnetAuth    types.Bool  `tfsdk:"telnet_auth"`
	TelnetPort    types.Int64 `tfsdk:"telnet_port"`
	SSHEnabled    types.Bool  `tfsdk:"ssh_enabled"`
	SSHAuth       types.Bool  `tfsdk:"ssh_auth"`
	SSHPort       types.Int64 `tfsdk:"ssh_port"`
	TCPEnabled    types.Bool  `tfsdk:"tcp_enabled"`
	TCPAuth       types.Bool  `tfsdk:"tcp_auth"`
	TCPPort       types.Int64 `tfsdk:"tcp_port"`
	IdleTimeout   types.Int64 `tfsdk:"idle_timeout"`
}

func (r *portAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_access"
}

func (r *portAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Network access services of one SLC Device Port (Telnet In / SSH In / raw TCP In). Destroying this resource disables all three services.",
		Attributes: map[string]schema.Attribute{
			"port": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Device Port number these access settings belong to.",
			},
			"telnet_enabled": schema.BoolAttribute{Optional: true, Computed: true, Description: "Telnet In service on this port."},
			"telnet_auth":    schema.BoolAttribute{Optional: true, Computed: true, Description: "Require SLC login before bridging (device default: enabled)."},
			"telnet_port":    schema.Int64Attribute{Optional: true, Computed: true, Description: "Listener override; default is the derived global base + port - 1."},
			"ssh_enabled":    schema.BoolAttribute{Optional: true, Computed: true, Description: "SSH In service on this port."},
			"ssh_auth":       schema.BoolAttribute{Optional: true, Computed: true, Description: "Require SLC login over SSH (device default: enabled)."},
			"ssh_port":       schema.Int64Attribute{Optional: true, Computed: true, Description: "Listener override."},
			"tcp_enabled":    schema.BoolAttribute{Optional: true, Computed: true, Description: "Raw TCP In service on this port."},
			"tcp_auth":       schema.BoolAttribute{Optional: true, Computed: true, Description: "Require SLC login for raw TCP (device default: disabled)."},
			"tcp_port":       schema.Int64Attribute{Optional: true, Computed: true, Description: "Listener override."},
			"idle_timeout":   schema.Int64Attribute{Optional: true, Computed: true, Description: "Idle disconnect in seconds (1-9999); 0 means disabled."},
		},
	}
}

func (r *portAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*slc.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("%T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *portAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n := int(plan.Port.ValueInt64())
	current, err := r.client.GetAccess(ctx, n)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read device port access", fmt.Sprint(err))
		return
	}
	applied, err := r.client.ApplyAccess(ctx, n, mergeAccess(current, plan))
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure device port access", fmt.Sprint(err))
		return
	}
	writeAccessState(&plan, applied)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *portAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetAccess(ctx, int(state.Port.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Cannot read device port access", fmt.Sprint(err))
		return
	}
	writeAccessState(&state, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n := int(plan.Port.ValueInt64())
	current, err := r.client.GetAccess(ctx, n)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read device port access", fmt.Sprint(err))
		return
	}
	applied, err := r.client.ApplyAccess(ctx, n, mergeAccess(current, plan))
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure device port access", fmt.Sprint(err))
		return
	}
	writeAccessState(&plan, applied)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete closes every access service on the port: releasing management must
// never leave a network door open behind Terraform (#9).
func (r *portAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state portAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DisableAccess(ctx, int(state.Port.ValueInt64())); err != nil {
		resp.Diagnostics.AddWarning("Could not disable access services", fmt.Sprint(err))
	}
}

func (r *portAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	n, err := strconv.Atoi(req.ID)
	if err != nil || n < 1 {
		resp.Diagnostics.AddError("Invalid import ID", `Expected a decimal device port number, e.g. "47".`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("port"), int64(n))...)
}

func mergeAccess(current slc.Access, plan portAccessModel) slc.Access {
	out := current
	if !plan.TelnetEnabled.IsNull() && !plan.TelnetEnabled.IsUnknown() {
		out.TelnetEnabled = plan.TelnetEnabled.ValueBool()
	}
	if !plan.TelnetAuth.IsNull() && !plan.TelnetAuth.IsUnknown() {
		out.TelnetAuth = plan.TelnetAuth.ValueBool()
	}
	if !plan.TelnetPort.IsNull() && !plan.TelnetPort.IsUnknown() {
		out.TelnetPort = int(plan.TelnetPort.ValueInt64())
	}
	if !plan.SSHEnabled.IsNull() && !plan.SSHEnabled.IsUnknown() {
		out.SSHEnabled = plan.SSHEnabled.ValueBool()
	}
	if !plan.SSHAuth.IsNull() && !plan.SSHAuth.IsUnknown() {
		out.SSHAuth = plan.SSHAuth.ValueBool()
	}
	if !plan.SSHPort.IsNull() && !plan.SSHPort.IsUnknown() {
		out.SSHPort = int(plan.SSHPort.ValueInt64())
	}
	if !plan.TCPEnabled.IsNull() && !plan.TCPEnabled.IsUnknown() {
		out.TCPEnabled = plan.TCPEnabled.ValueBool()
	}
	if !plan.TCPAuth.IsNull() && !plan.TCPAuth.IsUnknown() {
		out.TCPAuth = plan.TCPAuth.ValueBool()
	}
	if !plan.TCPPort.IsNull() && !plan.TCPPort.IsUnknown() {
		out.TCPPort = int(plan.TCPPort.ValueInt64())
	}
	if !plan.IdleTimeout.IsNull() && !plan.IdleTimeout.IsUnknown() {
		out.IdleTimeout = int(plan.IdleTimeout.ValueInt64())
	}
	return out
}

func writeAccessState(m *portAccessModel, a slc.Access) {
	m.Port = types.Int64Value(int64(a.Port))
	m.TelnetEnabled = types.BoolValue(a.TelnetEnabled)
	m.TelnetAuth = types.BoolValue(a.TelnetAuth)
	m.TelnetPort = types.Int64Value(int64(a.TelnetPort))
	m.SSHEnabled = types.BoolValue(a.SSHEnabled)
	m.SSHAuth = types.BoolValue(a.SSHAuth)
	m.SSHPort = types.Int64Value(int64(a.SSHPort))
	m.TCPEnabled = types.BoolValue(a.TCPEnabled)
	m.TCPAuth = types.BoolValue(a.TCPAuth)
	m.TCPPort = types.Int64Value(int64(a.TCPPort))
	m.IdleTimeout = types.Int64Value(int64(a.IdleTimeout))
}
