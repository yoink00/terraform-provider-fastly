package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var gcsloggingSchema = &schema.Schema{
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
			"email": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_GCS_EMAIL", ""),
				Description: "The email address associated with the target GCS bucket on your account.",
			},
			"bucket_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the bucket in which to store the logs.",
			},
			"secret_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_GCS_SECRET_KEY", ""),
				Description: "The secret key associated with the target gcs bucket on your account.",
				Sensitive:   true,
			},
			// Optional fields
			"path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to store the files. Must end with a trailing slash",
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
			"timestamp_format": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "%Y-%m-%dT%H:%M:%S.000",
				Description: "specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
			},
			"response_condition": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Name of a condition to apply this logging.",
			},
			"message_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "classic",
				Description: "The log message type per the fastly docs: https://docs.fastly.com/api/logging#logging_gcs",
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

func flattenGCS(gcsList []*fastly.GCS) []map[string]interface{} {
	var GCSList []map[string]interface{}
	for _, currentGCS := range gcsList {
		// Convert gcs to a map for saving to state.
		GCSMapString := map[string]interface{}{
			"name":               currentGCS.Name,
			"email":              currentGCS.User,
			"bucket_name":        currentGCS.Bucket,
			"secret_key":         currentGCS.SecretKey,
			"path":               currentGCS.Path,
			"period":             int(currentGCS.Period),
			"gzip_level":         int(currentGCS.GzipLevel),
			"response_condition": currentGCS.ResponseCondition,
			"message_type":       currentGCS.MessageType,
			"format":             currentGCS.Format,
			"timestamp_format":   currentGCS.TimestampFormat,
			"placement":          currentGCS.Placement,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range GCSMapString {
			if v == "" {
				delete(GCSMapString, k)
			}
		}

		GCSList = append(GCSList, GCSMapString)
	}

	return GCSList
}
