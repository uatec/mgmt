# Workspace reports

Put a Go `text/template` named `<name>.tmpl` here and run `mgmt report <name>`.
A template with the name of a built-in report (prep, review, explain, status,
status-yaml) overrides it. Templates see `.now`, `.person`, `.cycle`, `.view`, `.facts`
and `.derive`.
