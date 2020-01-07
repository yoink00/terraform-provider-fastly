---
layout: "fastly"
page_title: "Fastly: fastly_waf_rules"
sidebar_current: "docs-fastly-datasource-waf_rules"
description: |-
  Get information on Fastly WAF rules.
---

# fastly_waf_rules

Use this data source to get the [WAF rules][1] of Fastly.

## Simple Example Usage

```hcl
variable "type_status" {
  type = map(string)
  default = {
    score     = "score"
    threshold = "log"
    strict    = "log"
  }
}

source "fastly_service_v1" "myservice" {
  name = "demofastly"
  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }
  backend {
    address = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
    name    = "AWS S3 hosting"
    port    = 80
  }
  condition {
    name      = "WAF_Prefetch"
    type      = "PREFETCH"
    statement = "req.url~+\"index.html\""
  }
  response_object {
    name     = "WAF_Response"
    status   = "403"
    response = "Forbidden"
    content  = "content2"
  }
  waf {
    prefetch_condition = "WAF_Prefetch"
    response_object    = "WAF_Response"
  }
  force_destroy = true
}

data "fastly_waf_rules" "owasp" {
  publishers = ["owasp"]
}

resource "fastly_service_waf_configuration_v1" "waf" {
  waf_id                          = fastly_service_v1.foo.waf[0].waf_id
  http_violation_score_threshold  = 202

  dynamic "rule" {
    for_each = data.fastly_waf_rules.owasp.rules
    content {
      modsec_rule_id = rule.value.modsec_rule_id
      revision       = rule.value.latest_revision_number
      status         = lookup(var.type_status, rule.value.type, "log")
    }
  }
}
```

## Example Usage by ModSecurity ID

```hcl
variable "type_status" {
  type = map(string)
  default = {
    score     = "score"
    threshold = "log"
    strict    = "log"
  }
}

variable "individual_rules" {
  type = map(string)
  default = {
    1010020 = "block"
  }
}

source "fastly_service_v1" "myservice" {
  name = "demofastly"
  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }
  backend {
    address = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
    name    = "AWS S3 hosting"
    port    = 80
  }
  condition {
    name      = "WAF_Prefetch"
    type      = "PREFETCH"
    statement = "req.url~+\"index.html\""
  }
  response_object {
    name     = "WAF_Response"
    status   = "403"
    response = "Forbidden"
    content  = "content2"
  }
  waf {
    prefetch_condition = "WAF_Prefetch"
    response_object    = "WAF_Response"
  }
  force_destroy = true
}

data "fastly_waf_rules" "owasp" {
  publishers = ["owasp"]
}

resource "fastly_service_waf_configuration_v1" "waf" {
  waf_id                          = fastly_service_v1.foo.waf[0].waf_id
  http_violation_score_threshold  = 202

  dynamic "rule" {
    for_each = data.fastly_waf_rules.owasp.rules
    content {
      modsec_rule_id = rule.value.modsec_rule_id
      revision       = rule.value.latest_revision_number
      status         = lookup(var.individual_rules, rule.value.modsec_rule_id, lookup(var.type_status, rule.value.type, "log"))
    }
  }
}
```

## Attributes Reference

* `publishers` - Inclusion filter by WAF rule publishers.
* `tags` - Inclusion filter by WAF rules tags.
* `exclude_modsec_rule_ids` - Exclusion filter by WAF rule ModSecurity ID.

[1]: https://docs.fastly.com/guides/securing-communications/accessing-fastlys-ip-ranges
