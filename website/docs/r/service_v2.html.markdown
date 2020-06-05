---
layout: "fastly"
page_title: "Fastly: service_v1"
sidebar_current: "docs-fastly-resource-service-v1"
description: |-
  Provides an Fastly Service
---

# fastly_service_v1

Provides a Fastly Service, representing the configuration for a website, app,
API, or anything else to be served through Fastly. A Service encompasses Domains
and Backends.

The Service resource requires a domain name that is correctly set up to direct
traffic to the Fastly service. See Fastly's guide on [Adding CNAME Records][fastly-cname]
on their documentation site for guidance.

## Example Usage

Basic usage:

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "localhost"
    port    = 80
  }

  force_destroy = true
}
```

Basic usage with an Amazon S3 Website and that removes the `x-amz-request-id` header:

```hcl
resource "fastly_service_v1" "demo" {
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

  header {
    destination = "http.x-amz-request-id"
    type        = "cache"
    action      = "delete"
    name        = "remove x-amz-request-id"
  }

  gzip {
    name          = "file extensions and content types"
    extensions    = ["css", "js"]
    content_types = ["text/html", "text/css"]
  }

  default_host = "${aws_s3_bucket.website.name}.s3-website-us-west-2.amazonaws.com"

  force_destroy = true
}

resource "aws_s3_bucket" "website" {
  bucket = "demo.notexample.com"
  acl    = "public-read"

  website {
    index_document = "index.html"
    error_document = "error.html"
  }
}
```

Basic usage with [custom
VCL](https://docs.fastly.com/guides/vcl/uploading-custom-vcl) (must be
enabled on your Fastly account):

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "localhost"
    port    = 80
  }

  force_destroy = true

  vcl {
    name    = "my_custom_main_vcl"
    content = "${file("${path.module}/my_custom_main.vcl")}"
    main    = true
  }

  vcl {
    name    = "my_custom_library_vcl"
    content = "${file("${path.module}/my_custom_library.vcl")}"
  }
}
```

Basic usage with [custom Director](https://docs.fastly.com/api/config#director):

```hcl
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

  backend {
    address = "127.0.0.2"
    name    = "origin2"
    port    = 80
  }

  director {
    name = "mydirector"
    quorum = 0
    type = 3
    backends = [ "origin1", "origin2" ]
  }

  force_destroy = true
}
```

-> **Note:** For an AWS S3 Bucket, the Backend address is
`<domain>.s3-website-<region>.amazonaws.com`. The `default_host` attribute
should be set to `<bucket_name>.s3-website-<region>.amazonaws.com`. See the
Fastly documentation on [Amazon S3][fastly-s3].

## Argument Reference

The following arguments are supported:

* `activate` - (Optional) Conditionally prevents the Service from being activated. The apply step will continue to create a new draft version but will not activate it if this is set to false. Default true.
* `name` - (Required) The unique name for the Service to create.
* `comment` - (Optional) Description field for the service. Default `Managed by Terraform`.
* `version_comment` - (Optional) Description field for the version.
* `domain` - (Required) A set of Domain names to serve as entry points for your
Service. Defined below.
* `backend` - (Optional) A set of Backends to service requests from your Domains.
Defined below. Backends must be defined in this argument, or defined in the
`vcl` argument below
* `healthcheck` - (Optional) Automated healthchecks on the cache that can change how Fastly interacts with the cache based on its health.
* `force_destroy` - (Optional) Services that are active cannot be destroyed. In
order to destroy the Service, set `force_destroy` to `true`. Default `false`.
* `s3logging` - (Optional) A set of S3 Buckets to send streaming logs too.
Defined below.
* `papertrail` - (Optional) A Papertrail endpoint to send streaming logs too.
Defined below.
* `sumologic` - (Optional) A Sumologic endpoint to send streaming logs too.
Defined below.
* `gcslogging` - (Optional) A gcs endpoint to send streaming logs too.
Defined below.
* `bigquerylogging` - (Optional) A BigQuery endpoint to send streaming logs too.
Defined below.
* `syslog` - (Optional) A syslog endpoint to send streaming logs too.
Defined below.
* `logentries` - (Optional) A logentries endpoint to send streaming logs too.
Defined below.
* `splunk` - (Optional) A Splunk endpoint to send streaming logs too.
Defined below.
* `blobstoragelogging` - (Optional) An Azure Blob Storage endpoint to send streaming logs too.
Defined below.
* `httpslogging` - (Optional) An HTTPS endpoint to send streaming logs to.
Defined below.

The `domain` block supports:

* `name` - (Required) The domain to which this Service will respond.
* `comment` - (Optional) An optional comment about the Domain.

The `backend` block supports:

* `name` - (Required, string) Name for this Backend. Must be unique to this Service.
* `address` - (Required, string) An IPv4, hostname, or IPv6 address for the Backend.
* `auto_loadbalance` - (Optional, boolean) Denotes if this Backend should be
included in the pool of backends that requests are load balanced against.
Default `true`.
* `between_bytes_timeout` - (Optional) How long to wait between bytes in milliseconds. Default `10000`.
* `connect_timeout` - (Optional) How long to wait for a timeout in milliseconds.
Default `1000`
* `error_threshold` - (Optional) Number of errors to allow before the Backend is marked as down. Default `0`.
* `first_byte_timeout` - (Optional) How long to wait for the first bytes in milliseconds. Default `15000`.
* `max_conn` - (Optional) Maximum number of connections for this Backend.
Default `200`.
* `port` - (Optional) The port number on which the Backend responds. Default `80`.
* `override_host` - (Optional) The hostname to override the Host header.
* `request_condition` - (Optional, string) Name of already defined `condition`, which if met, will select this backend during a request.
* `use_ssl` - (Optional) Whether or not to use SSL to reach the backend. Default `false`.
* `max_tls_version` - (Optional) Maximum allowed TLS version on SSL connections to this backend.
* `min_tls_version` - (Optional) Minimum allowed TLS version on SSL connections to this backend.
* `ssl_ciphers` - (Optional) Comma separated list of OpenSSL Ciphers to try when negotiating to the backend.
* `ssl_ca_cert` - (Optional) CA certificate attached to origin.
* `ssl_client_cert` - (Optional) Client certificate attached to origin. Used when connecting to the backend.
* `ssl_client_key` - (Optional) Client key attached to origin. Used when connecting to the backend.
* `ssl_check_cert` - (Optional) Be strict about checking SSL certs. Default `true`.
* `ssl_hostname` - (Optional, deprecated by Fastly) Used for both SNI during the TLS handshake and to validate the cert.
* `ssl_cert_hostname` - (Optional) Overrides ssl_hostname, but only for cert verification. Does not affect SNI at all.
* `ssl_sni_hostname` - (Optional) Overrides ssl_hostname, but only for SNI in the handshake. Does not affect cert validation at all.
* `shield` - (Optional) The POP of the shield designated to reduce inbound load. Valid values for `shield` are included in the [`GET /datacenters`](https://docs.fastly.com/api/tools#datacenter) API response.
* `weight` - (Optional) The [portion of traffic](https://docs.fastly.com/guides/performance-tuning/load-balancing-configuration.html#how-weight-affects-load-balancing) to send to this Backend. Each Backend receives `weight / total` of the traffic. Default `100`.
* `healthcheck` - (Optional) Name of a defined `healthcheck` to assign to this backend.


The `healthcheck` block supports:

* `name` - (Required) A unique name to identify this Healthcheck.
* `host` - (Required) The Host header to send for this Healthcheck.
* `path` - (Required) The path to check.
* `check_interval` - (Optional) How often to run the Healthcheck in milliseconds. Default `5000`.
* `expected_response` - (Optional) The status code expected from the host. Default `200`.
* `http_version` - (Optional) Whether to use version 1.0 or 1.1 HTTP. Default `1.1`.
* `initial` - (Optional) When loading a config, the initial number of probes to be seen as OK. Default `2`.
* `method` - (Optional) Which HTTP method to use. Default `HEAD`.
* `threshold` - (Optional) How many Healthchecks must succeed to be considered healthy. Default `3`.
* `timeout` - (Optional) Timeout in milliseconds. Default `500`.
* `window` - (Optional) The number of most recent Healthcheck queries to keep for this Healthcheck. Default `5`.

The `s3logging` block supports:

* `name` - (Required) A unique name to identify this S3 Logging Bucket.
* `bucket_name` - (Required) The name of the bucket in which to store the logs.
* `s3_access_key` - (Required) AWS Access Key of an account with the required
permissions to post logs. It is **strongly** recommended you create a separate
IAM user with permissions to only operate on this Bucket. This key will be
not be encrypted. You can provide this key via an environment variable, `FASTLY_S3_ACCESS_KEY`.
* `s3_secret_key` - (Required) AWS Secret Key of an account with the required
permissions to post logs. It is **strongly** recommended you create a separate
IAM user with permissions to only operate on this Bucket. This secret will be
not be encrypted. You can provide this secret via an environment variable, `FASTLY_S3_SECRET_KEY`.
* `path` - (Optional) Path to store the files. Must end with a trailing slash.
If this field is left empty, the files will be saved in the bucket's root path.
* `domain` - (Optional) If you created the S3 bucket outside of `us-east-1`,
then specify the corresponding bucket endpoint. Example: `s3-us-west-2.amazonaws.com`.
* `period` - (Optional) How frequently the logs should be transferred, in
seconds. Default `3600`.
* `gzip_level` - (Optional) Level of GZIP compression, from `0-9`. `0` is no
compression. `1` is fastest and least compressed, `9` is slowest and most
compressed. Default `0`.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Defaults to Apache Common Log format (`%h %l %u %t %r %>s`)
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either 1 (the default, version 1 log format) or 2 (the version 2 log format).
* `message_type` - (Optional) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`.  Default `classic`.
* `timestamp_format` - (Optional) `strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`).
* `redundancy` - (Optional) The S3 redundancy level. Should be formatted; one of: `standard`, `reduced_redundancy` or null. Default `null`.
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals,
see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.

The `papertrail` block supports:

* `name` - (Required) A unique name to identify this Papertrail endpoint.
* `address` - (Required) The address of the Papertrail endpoint.
* `port` - (Required) The port associated with the address where the Papertrail endpoint can be accessed.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Defaults to Apache Common Log format (`%h %l %u %t %r %>s`)
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals,
see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.

The `sumologic` block supports:

* `name` - (Required) A unique name to identify this Sumologic endpoint.
* `url` - (Required) The URL to Sumologic collector endpoint
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Defaults to Apache Common Log format (`%h %l %u %t %r %>s`)
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either 1 (the default, version 1 log format) or 2 (the version 2 log format).
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `message_type` - (Optional) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. See [Fastly's Documentation on Sumologic][fastly-sumologic]
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.


The `gcslogging` block supports:

* `name` - (Required) A unique name to identify this GCS endpoint.
* `email` - (Required) The email address associated with the target GCS bucket on your account. You may optionally provide this secret via an environment variable, `FASTLY_GCS_EMAIL`.
* `bucket_name` - (Required) The name of the bucket in which to store the logs.
* `secret_key` - (Required) The secret key associated with the target gcs bucket on your account. You may optionally provide this secret via an environment variable, `FASTLY_GCS_SECRET_KEY`. A typical format for the key is PEM format, containing actual newline characters where required.
* `path` - (Optional) Path to store the files. Must end with a trailing slash.
If this field is left empty, the files will be saved in the bucket's root path.
* `period` - (Optional) How frequently the logs should be transferred, in
seconds. Default `3600`.
* `gzip_level` - (Optional) Level of GZIP compression, from `0-9`. `0` is no
compression. `1` is fastest and least compressed, `9` is slowest and most
compressed. Default `0`.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Defaults to Apache Common Log format (`%h %l %u %t %r %>s`)
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `message_type` - (Optional) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. [Fastly Documentation](https://docs.fastly.com/api/logging#logging_gcs)
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.


The `bigquerylogging` block supports:

* `name` - (Required) A unique name to identify this BigQuery logging endpoint.
* `project_id` - (Required) The ID of your GCP project.
* `dataset` - (Required) The ID of your BigQuery dataset.
* `table` - (Required) The ID of your BigQuery table.
* `email` - (Optional) The email for the service account with write access to your BigQuery dataset. If not provided, this will be pulled from a `FASTLY_BQ_EMAIL` environment variable.
* `secret_key` - (Optional) The secret key associated with the sservice account that has write access to your BigQuery table. If not provided, this will be pulled from the `FASTLY_BQ_SECRET_KEY` environment variable. Typical format for this is a private key in a string with newlines.
* `format` - (Optional) Apache style log formatting. Must produce JSON that matches the schema of your BigQuery table.
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `template` - (Optional) Big query table name suffix template. If set will be interpreted as a strftime compatible string and used as the [Template Suffix for your table](https://cloud.google.com/bigquery/streaming-data-into-bigquery#template-tables).
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.


The `syslog` block supports:

* `name` - (Required) A unique name to identify this Syslog endpoint.
* `address` - (Required) A hostname or IPv4 address of the Syslog endpoint.
* `port` - (Optional) The port associated with the address where the Syslog endpoint can be accessed. Default `514`.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Defaults to Apache Common Log format (%h %l %u %t %r %>s)
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either 1 (the default, version 1 log format) or 2 (the version 2 log format).
* `token` - (Optional) Whether to prepend each message with a specific token.
* `use_tls` - (Optional) Whether to use TLS for secure logging. Default `false`.
* `tls_hostname` - (Optional) Used during the TLS handshake to validate the certificate.
* `tls_ca_cert` - (Optional) A secure certificate to authenticate the server with. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SYSLOG_CA_CERT`
* `tls_client_cert` - (Optional) The client certificate used to make authenticated requests. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SYSLOG_CLIENT_CERT`
* `tls_client_key` - (Optional) The client private key used to make authenticated requests. Must be in PEM format. You can provide this key via an environment variable, `FASTLY_SYSLOG_CLIENT_KEY`
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals,
see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `message_type` - (Optional) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`.  Default `classic`.
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.


The `logentries` block supports:

* `name` - (Required) A unique name to identify this GCS endpoint.
* `token` - (Required) Logentries Token to be used for authentication (https://logentries.com/doc/input-token/).
* `port` - (Optional) The port number configured in Logentries to send logs to. Defaults to `20000`.
* `use_tls` - (Optional) Whether to use TLS for secure logging. Defaults to `true`
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Defaults to Apache Common Log format (`%h %l %u %t %r %>s`).
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either 1 (the default, version 1 log format) or 2 (the version 2 log format).
* `response_condition` - (Optional) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals][fastly-conditionals].
* `placement` - (Optional) Where in the generated VCL the logging call should be placed; one of: `none` or `waf_debug`.


The `blobstoragelogging` block supports:

* `name` - (Required) A unique name to identify the Azure Blob Storage endpoint.
* `account_name` - (Required) The unique Azure Blob Storage namespace in which your data objects are stored.
* `container` - (Required) The name of the Azure Blob Storage container in which to store logs.
* `sas_token` - (Required) The Azure shared access signature providing write access to the blob service objects. Be sure to update your token before it expires or the logging functionality will not work.
* `path` - (Optional) The path to upload logs to. Must end with a trailing slash. If this field is left empty, the files will be saved in the container's root path.
* `period` - (Optional) How frequently the logs should be transferred in seconds. Default `3600`.
* `timestamp_format` - (Optional) `strftime` specified timestamp formatting. Default `%Y-%m-%dT%H:%M:%S.000`.
* `gzip_level` - (Optional) Level of GZIP compression from `0`to `9`. `0` means no compression. `1` is the fastest and the least compressed version, `9` is the slowest and the most compressed version. Default `0`.
* `public_key` - (Optional) A PGP public key that Fastly will use to encrypt your log files before writing them to disk.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Default `%h %l %u %t \"%r\" %>s %b`.
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`. Default `2`.
* `message_type` - (Optional) How the message should be formatted. Can be either `classic`, `loggly`, `logplex` or `blank`.  Default `classic`.
* `placement` - (Optional) Where in the generated VCL the logging call should be placed, overriding any `format_version` default. Can be either `none` or `waf_debug`.
* `response_condition` - (Optional) The name of the `condition` to apply. If empty, always execute.


The `splunk` block supports:

* `name` - (Required) A unique name to identify the Splunk endpoint.
* `url` - (Required) The Splunk URL to stream logs to.
* `token` - (Required) The Splunk token to be used for authentication.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting. Default `%h %l %u %t \"%r\" %>s %b`.
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`. Default `2`.
* `placement` - (Optional) Where in the generated VCL the logging call should be placed, overriding any `format_version` default. Can be either `none` or `waf_debug`.
* `response_condition` - (Optional) The name of the `condition` to apply. If empty, always execute.
* `tls_hostname` - (Optional) The hostname used to verify the server's certificate. It can either be the Common Name or a Subject Alternative Name (SAN).
* `tls_ca_cert` - (Optional) A secure certificate to authenticate the server with. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SPLUNK_CA_CERT`.


The `httpslogging` block supports:

* `name` - (Required) The unique name of the HTTPS logging endpoint.
* `url` - (Required) URL that log data will be sent to. Must use the https protocol.
* `request_max_entries` - (Optional) The maximum number of logs sent in one request.
* `request_max_bytes` - (Optional) The maximum number of bytes sent in one request.
* `content_type` - (Optional) Value of the `Content-Type` header sent with the request.
* `header_name` - (Optional) Custom header sent with the request.
* `header_value` - (Optional) Value of the custom header sent with the request.
* `method` - (Optional) HTTP method used for request. Can be either `POST` or `PUT`. Default `POST`.
* `json_format` - Formats log entries as JSON. Can be either disabled (`0`), array of json (`1`), or newline delimited json (`2`).
* `tls_hostname` - (Optional) Used during the TLS handshake to validate the certificate.
* `tls_ca_cert` - (Optional) A secure certificate to authenticate the server with. Must be in PEM format.
* `tls_client_cert` - (Optional) The client certificate used to make authenticated requests. Must be in PEM format.
* `tls_client_key` - (Optional) The client private key used to make authenticated requests. Must be in PEM format.
* `format` - (Optional) Apache-style string or VCL variables to use for log formatting.
* `format_version` - (Optional) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`. Default `2`.
* `message_type` - How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`.  Default `blank`.
* `placement` - (Optional) Where in the generated VCL the logging call should be placed.
* `response_condition` - (Optional) The name of the `condition` to apply. If empty, always execute.


## Attributes Reference

In addition to the arguments listed above, the following attributes are exported:

* `id` – The ID of the Service.
* `active_version` – The currently active version of your Fastly Service.
* `cloned_version` - The latest cloned version by the provider. The value gets only set after running `terraform apply`.



## Import

Fastly Service can be imported using their service ID, e.g.

```
$ terraform import fastly_service_v1.demo xxxxxxxxxxxxxxxxxxxx
```
