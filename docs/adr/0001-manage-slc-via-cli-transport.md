# 1. Manage the SLC via CLI session transports, not the web/SNMP APIs

The provider drives the SLC by scripting its interactive CLI over a Transport (Telnet first), rather than using the web interface, SNMP, or Lantronix's NetConfig/LMS discovery-and-config protocols.

Considered alternatives: the web UI (HTML scraping, auth churn, no serial fallback) and NetConfig/SNMP (binary/UDP protocols that can't reach a device whose network settings are broken). We chose the CLI because the exact same dialect runs over Telnet, SSH, and the local serial console — so a device that has lost its network config is still manageable, and adding transports later doesn't touch the resource logic.

Consequences: we must build and maintain robust session handling (login, prompts, paging) per transport, and parsing is text-based rather than structured.
