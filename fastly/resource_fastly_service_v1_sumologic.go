package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var sumologicSchema = &schema.Schema{
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
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The URL to POST to.",
			},
			// Optional fields
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

func flattenSumologics(sumologicList []*fastly.Sumologic) []map[string]interface{} {
	var l []map[string]interface{}
	for _, p := range sumologicList {
		// Convert Sumologic to a map for saving to state.
		ns := map[string]interface{}{
			"name":               p.Name,
			"url":                p.URL,
			"format":             p.Format,
			"response_condition": p.ResponseCondition,
			"message_type":       p.MessageType,
			"format_version":     int(p.FormatVersion),
			"placement":          p.Placement,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range ns {
			if v == "" {
				delete(ns, k)
			}
		}

		l = append(l, ns)
	}

	return l
}
