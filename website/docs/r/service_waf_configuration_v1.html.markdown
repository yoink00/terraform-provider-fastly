---
layout: "fastly"
page_title: "Fastly: service_waf_configuration_v1"
sidebar_current: "docs-fastly-resource-service-waf-configuration-v1"
description: |-
  Provides a Web Application Firewall configuration and rules that can be applied to a service. 
---

# fastly_service_waf_configuration_v1

Defines a set a Web Application Firewall configuration options that can be used to populate a service waf.  This resource will populate a waf with configuration and waf rules and will track their state.


~> **Warning:** Terraform will take precedence over any changes you make in the UI or API. Such changes are likely to be reversed if you run Terraform again.  

If Terraform is being used to populate the initial content of a dictionary which you intend to manage via API or UI, then the lifecycle `ignore_changes` field can be used with the resource.  An example of this configuration is provided below.    


## Example Usage

Basic usage:

```hcl
variable "type_status" {
  type = map(string)
  default = {
    score     = "score"
    threshold = "log"
    strict    = "log"
  }
}

resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "origin1"
    port    = 80
  }

  condition {
    name      = "Waf_Prefetch"
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
    prefetch_condition = "Waf_Prefetch"
    response_object    = "WAF_Response"
  }

  force_destroy = true
}

resource "fastly_service_waf_configuration_v1" "waf" {

  waf_id                          = fastly_service_v1.foo.waf[0].waf_id
  http_violation_score_threshold  = 100

  dynamic "rule" {
    for_each = data.fastly_waf_rules.r1.rules

    content {
      modsec_rule_id = rule.value.modsec_rule_id
      revision       = rule.value.latest_revision_number
      status         = lookup(var.type_status, rule.value.type, "log")
    }
  }
}
```

## Argument Reference

The following arguments are supported:

* `waf_id` - (Required) The ID of the Web Application Firewall that the configuration belongs to
* `allowed_http_versions` - (Optional) Allowed HTTP versions (default HTTP/1.0 HTTP/1.1 HTTP/2)
* `allowed_methods` - (Optional) A space-separated list of HTTP method names (default GET HEAD POST OPTIONS PUT PATCH DELETE)
* `allowed_request_content_type` - (Optional) Allowed request content types (default application/x-www-form-urlencoded|multipart/form-data|text/xml|application/xml|application/x-amf|application/json|text/plain)
* `allowed_request_content_type_charset` - (Required) Allowed request content type charset (default utf-8|iso-8859-1|iso-8859-15|windows-1252)
* `arg_length` - (Optional) The maximum number of arguments allowed (default 400)
* `arg_name_length` - (Optional) The maximum allowed argument name length (default 100)
* `combined_file_sizes` - (Optional) The maximum allowed size of all files (in bytes, default 10000000)
* `critical_anomaly_score` - (Optional) Score value to add for critical anomalies (default 6)
* `crs_validate_utf8_encoding` - (Optional) CRS validate UTF8 encoding
* `error_anomaly_score` - (Optional) Score value to add for error anomalies (default 5)
* `high_risk_country_codes` - (Optional) A space-separated list of country codes in ISO 3166-1 (two-letter) format
* `http_violation_score_threshold` - (Optional) HTTP violation threshold
* `inbound_anomaly_score_threshold` - (Optional) Inbound anomaly threshold
* `lfi_score_threshold` - (Optional) Local file inclusion attack threshold
* `max_file_size` - (Optional) The maximum allowed file size, in bytes (default 10000000)
* `max_num_args` - (Optional) The maximum number of arguments allowed (default 255)
* `notice_anomaly_score` - (Optional) Score value to add for notice anomalies (default 4)
* `paranoia_level` - (Optional) The configured paranoia level (default 1)
* `php_injection_score_threshold` - (Optional) PHP injection threshold
* `rce_score_threshold` - (Optional) Remote code execution threshold
* `restricted_extensions` - (Optional) A space-separated list of allowed file extensions (default .asa/ .asax/ .ascx/ .axd/ .backup/ .bak/ .bat/ .cdx/ .cer/ .cfg/ .cmd/ .com/ .config/ .conf/ .cs/ .csproj/ .csr/ .dat/ .db/ .dbf/ .dll/ .dos/ .htr/ .htw/ .ida/ .idc/ .idq/ .inc/ .ini/ .key/ .licx/ .lnk/ .log/ .mdb/ .old/ .pass/ .pdb/ .pol/ .printer/ .pwd/ .resources/ .resx/ .sql/ .sys/ .vb/ .vbs/ .vbproj/ .vsdisco/ .webinfo/ .xsd/ .xsx)
* `restricted_headers` - (Optional) A space-separated list of allowed header names (default /proxy/ /lock-token/ /content-range/ /translate/ /if/)
* `rfi_score_threshold` - (Optional) Remote file inclusion attack threshold
* `session_fixation_score_threshold` - (Optional) Session fixation attack threshold
* `sql_injection_score_threshold` - (Optional) SQL injection attack threshold
* `total_arg_length` - (Optional) The maximum size of argument names and values (default 6400)
* `warning_anomaly_score` - (Optional) Score value to add for warning anomalies
* `xss_score_threshold` - (Optional) XSS attack threshold


## Attributes Reference

* [fastly-waf_configuration](https://docs.fastly.com/api/ngwaf#api-section-ngwaf_firewall_versions)

## Import
 
