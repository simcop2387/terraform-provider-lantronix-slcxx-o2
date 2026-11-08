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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/slc"
)

var (
	_ resource.Resource                = (*portResource)(nil)
	_ resource.ResourceWithImportState = (*portResource)(nil)
)

// NewPortResource manages serial settings of one Device Port.
func NewPortResource() resource.Resource { return &portResource{} }

type portResource struct{ client *slc.Client }

type portModel struct {
	Number      types.Int64  `tfsdk:"number"`
	Name        types.String `tfsdk:"name"`
	BaudRate    types.Int64  `tfsdk:"baud_rate"`
	DataBits    types.Int64  `tfsdk:"data_bits"`
	Parity      types.String `tfsdk:"parity"`
	StopBits    types.Int64  `tfsdk:"stop_bits"`
	FlowControl types.String `tfsdk:"flow_control"`
	Logins      types.Bool   `tfsdk:"logins"`
}

func (r *portResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port"
}

func (r *portResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Serial settings of one SLC Device Port. Creating a resource adopts the existing port; destroying it only forgets it.",
		Attributes: map[string]schema.Attribute{
			"number": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Device Port number (1-48 on an SLC48; the device validates the upper bound).",
			},
			"name": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Port label as shown by `show deviceport names`.",
			},
			"baud_rate":    schema.Int64Attribute{Optional: true, Computed: true, Description: "One of 300/600/1200/2400/4800/9600/19200/38400/57600/115200."},
			"data_bits":    schema.Int64Attribute{Optional: true, Computed: true, Description: "7 or 8."},
			"parity":       schema.StringAttribute{Optional: true, Computed: true, Description: "none, odd, or even."},
			"stop_bits":    schema.Int64Attribute{Optional: true, Computed: true, Description: "1 or 2."},
			"flow_control": schema.StringAttribute{Optional: true, Computed: true, Description: "none, xon/xoff, or rts/cts."},
			"logins":       schema.BoolAttribute{Optional: true, Computed: true, Description: "Serial-side SLC login prompt; must be false for network-endpoint use."},
		},
	}
}

func (r *portResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *portResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n := int(plan.Number.ValueInt64())
	current, err := r.client.GetSerial(ctx, n)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read device port", fmt.Sprint(err))
		return
	}
	applied, err := r.client.ApplySerial(ctx, n, mergeSerial(current, plan))
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure device port", fmt.Sprint(err))
		return
	}
	writeSerialState(&plan, applied)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *portResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetSerial(ctx, int(state.Number.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Cannot read device port", fmt.Sprint(err))
		return
	}
	writeSerialState(&state, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n := int(plan.Number.ValueInt64())
	current, err := r.client.GetSerial(ctx, n)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read device port", fmt.Sprint(err))
		return
	}
	applied, err := r.client.ApplySerial(ctx, n, mergeSerial(current, plan))
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure device port", fmt.Sprint(err))
		return
	}
	writeSerialState(&plan, applied)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete forgets the port: hardware ports outlive their management.
func (r *portResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *portResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	n, err := strconv.Atoi(req.ID)
	if err != nil || n < 1 {
		resp.Diagnostics.AddError("Invalid import ID", `Expected a decimal device port number, e.g. "47".`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("number"), int64(n))...)
}

// mergeSerial overlays declared (non-null) plan values onto the device's
// current values: unspecified attributes are preserved, not rewritten.
func mergeSerial(current slc.Serial, plan portModel) slc.Serial {
	out := current
	if !plan.BaudRate.IsNull() && !plan.BaudRate.IsUnknown() {
		out.Baud = int(plan.BaudRate.ValueInt64())
	}
	if !plan.DataBits.IsNull() && !plan.DataBits.IsUnknown() {
		out.DataBits = int(plan.DataBits.ValueInt64())
	}
	if !plan.Parity.IsNull() && !plan.Parity.IsUnknown() {
		out.Parity = plan.Parity.ValueString()
	}
	if !plan.StopBits.IsNull() && !plan.StopBits.IsUnknown() {
		out.StopBits = int(plan.StopBits.ValueInt64())
	}
	if !plan.FlowControl.IsNull() && !plan.FlowControl.IsUnknown() {
		out.FlowControl = plan.FlowControl.ValueString()
	}
	if !plan.Logins.IsNull() && !plan.Logins.IsUnknown() {
		out.Logins = plan.Logins.ValueBool()
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		out.Name = plan.Name.ValueString()
	}
	return out
}

func writeSerialState(m *portModel, s slc.Serial) {
	m.Number = types.Int64Value(int64(s.Number))
	m.Name = types.StringValue(s.Name)
	m.BaudRate = types.Int64Value(int64(s.Baud))
	m.DataBits = types.Int64Value(int64(s.DataBits))
	m.Parity = types.StringValue(s.Parity)
	m.StopBits = types.Int64Value(int64(s.StopBits))
	m.FlowControl = types.StringValue(s.FlowControl)
	m.Logins = types.BoolValue(s.Logins)
}
