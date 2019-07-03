package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var s3loggingSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			// Required fields
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name to refer to this logging setup",
			},
			"bucket_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "S3 Bucket name to store logs in",
			},
			"s3_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_S3_ACCESS_KEY", ""),
				Description: "AWS Access Key",
				Sensitive:   true,
			},
			"s3_secret_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_S3_SECRET_KEY", ""),
				Description: "AWS Secret Key",
				Sensitive:   true,
			},
			// Optional fields
			"path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to store the files. Must end with a trailing slash",
			},
			"domain": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Bucket endpoint",
				Default:     "s3.amazonaws.com",
			},
			"gzip_level": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Gzip Compression level",
			},
			"period": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     3600,
				Description: "How frequently the logs should be transferred, in seconds (Default 3600)",
			},
			"format": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "%h %l %u %t %r %>s",
				Description: "Apache-style string or VCL variables to use for log formatting",
			},
			"format_version": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1,
				Description:  "The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (Default: 1)",
				ValidateFunc: validateLoggingFormatVersion(),
			},
			"timestamp_format": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "%Y-%m-%dT%H:%M:%S.000",
				Description: "specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
			},
			"redundancy": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The S3 redundancy level.",
			},
			"response_condition": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Name of a condition to apply this logging.",
			},
			"message_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "classic",
				Description:  "How the message should be formatted.",
				ValidateFunc: validateLoggingMessageType(),
			},
			"placement": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Where in the generated VCL the logging call should be placed.",
				ValidateFunc: validateLoggingPlacement(),
			},
		},
	},
}

func flattenS3s(s3List []*fastly.S3) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, s := range s3List {
		// Convert S3s to a map for saving to state.
		ns := map[string]interface{}{
			"name":               s.Name,
			"bucket_name":        s.BucketName,
			"s3_access_key":      s.AccessKey,
			"s3_secret_key":      s.SecretKey,
			"path":               s.Path,
			"period":             s.Period,
			"domain":             s.Domain,
			"gzip_level":         s.GzipLevel,
			"format":             s.Format,
			"format_version":     s.FormatVersion,
			"timestamp_format":   s.TimestampFormat,
			"redundancy":         s.Redundancy,
			"response_condition": s.ResponseCondition,
			"message_type":       s.MessageType,
			"placement":          s.Placement,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range ns {
			if v == "" {
				delete(ns, k)
			}
		}

		sl = append(sl, ns)
	}

	return sl
}
