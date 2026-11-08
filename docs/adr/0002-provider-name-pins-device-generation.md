# 2. Provider type name `lantronix_slcxx_o2` pins the device generation

The Terraform provider type name is `lantronix_slcxx_o2` (HCL: `provider "lantronix_slcxx_o2"`, resources `lantronix_slcxx_o2_*`). Provider names must match `[a-z0-9_]+`, and the public registries are a shared namespace.

Lantronix ships multiple console-server generations (this O2-era SecureLinx family — SLC08/16/24/48 — plus O3 and modern ConsoleBox/ES lines), and their configuration surfaces differ enough that a separate provider may be warranted for each. Rather than claim the generic `lantronix` / `slc` names and collide with (or be forced to absorb) providers for other generations, we pin this provider to the `o2` generation now.

Consequences: resource addresses are verbose forever; if the CLI dialect turns out identical across generations, widening this provider is easy, but renaming or splitting a published provider later would be breaking. Extending to genuinely different generations means a new provider, not a rename.
