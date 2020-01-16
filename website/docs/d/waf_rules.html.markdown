---
layout: "fastly"
page_title: "Fastly: fastly_waf_rules"
sidebar_current: "docs-fastly-datasource-waf_rules"
description: |-
  Get information on Fastly WAF rules.
---

-> **Note:** This data source is only available from x.x.x of the Fastly terraform provider.

# fastly_waf_rules

Use this data source to get the [WAF rules][1] of Fastly.

## Example Usage

Usage with publishers Filter:

```hcl
data "fastly_waf_rules" "owasp" {
  publishers = ["owasp"]
}
```

Usage with tags filter:

```hcl
data "fastly_waf_rules" "tag" {
  tags = ["language-html", "language-jsp"]
}
```

Usage with exclude filter:

```hcl
data "fastly_waf_rules" "owasp_with_exclusions" {
  publishers = ["owasp"]
  exclude_modsec_rule_ids = [1010090]
}
```

Usage without filters:

```hcl
data "fastly_waf_rules" "all" {
  // This will retrieve the entire rule list available on the server at the time.
}
```

## Argument Reference

~> **Warning:** The data source's filters are applied using an **AND** boolean operator, so depending on the combination of those, 
they may become mutually exclusive.

* `publishers` - Inclusion filter by WAF rule's publishers.
* `tags` - Inclusion filter by WAF rule's tags.
* `exclude_modsec_rule_ids` - Exclusion filter by WAF rule's ModSecurity ID.

## Attribute Reference

* `rules` - The Web Application Firewall's active rules.

The `rules` block supports:

* `modsec_rule_id` - The rule's modsecurity ID.
* `latest_revision_number` - The rule's latest revision.
* `type` - The rule's type.

[1]: https://docs.fastly.com/en/guides/fastly-waf-rule-set-updates-maintenance
