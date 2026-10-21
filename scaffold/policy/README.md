# Workspace policies

Drop `*.rego` files here; each loads alongside the core policies, with no
rebuild. A policy is a package `mgmt.policy.<name>` that may read
`data.mgmt.derive` and emits `deny` (errors) and `warn` (warnings) sets of
`{id, person, file, line, message}`.
