package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var healthcheckSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			// required fields
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A name to refer to this healthcheck",
			},
			"host": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Which host to check",
			},
			"path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The path to check",
			},
			// optional fields
			"check_interval": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     5000,
				Description: "How often to run the healthcheck in milliseconds",
			},
			"expected_response": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     200,
				Description: "The status code expected from the host",
			},
			"http_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "1.1",
				Description: "Whether to use version 1.0 or 1.1 HTTP",
			},
			"initial": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     2,
				Description: "When loading a config, the initial number of probes to be seen as OK",
			},
			"method": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "HEAD",
				Description: "Which HTTP method to use",
			},
			"threshold": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     3,
				Description: "How many healthchecks must succeed to be considered healthy",
			},
			"timeout": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     500,
				Description: "Timeout in milliseconds",
			},
			"window": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     5,
				Description: "The number of most recent healthcheck queries to keep for this healthcheck",
			},
		},
	},
}

func flattenHealthchecks(healthcheckList []*fastly.HealthCheck) []map[string]interface{} {
	var hl []map[string]interface{}
	for _, h := range healthcheckList {
		// Convert HealthChecks to a map for saving to state.
		nh := map[string]interface{}{
			"name":              h.Name,
			"host":              h.Host,
			"path":              h.Path,
			"check_interval":    h.CheckInterval,
			"expected_response": h.ExpectedResponse,
			"http_version":      h.HTTPVersion,
			"initial":           h.Initial,
			"method":            h.Method,
			"threshold":         h.Threshold,
			"timeout":           h.Timeout,
			"window":            h.Window,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nh {
			if v == "" {
				delete(nh, k)
			}
		}

		hl = append(hl, nh)
	}

	return hl
}
