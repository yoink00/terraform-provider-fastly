package fastly

import "github.com/hashicorp/terraform-plugin-sdk/helper/schema"

var requestSettingSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			// Required fields
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name to refer to this Request Setting",
			},
			// Optional fields
			"request_condition": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Name of a request condition to apply. If there is no condition this setting will always be applied.",
			},
			"max_stale_age": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "How old an object is allowed to be, in seconds. Default `60`",
			},
			"force_miss": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Force a cache miss for the request",
			},
			"force_ssl": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Forces the request use SSL",
			},
			"action": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Allows you to terminate request handling and immediately perform an action",
			},
			"bypass_busy_wait": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Disable collapsed forwarding",
			},
			"hash_keys": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Comma separated list of varnish request object fields that should be in the hash key",
			},
			"xff": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "append",
				Description: "X-Forwarded-For options",
			},
			"timer_support": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Injects the X-Timer info into the request",
			},
			"geo_headers": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Inject Fastly-Geo-Country, Fastly-Geo-City, and Fastly-Geo-Region",
			},
			"default_host": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "the host header",
			},
		},
	},
}

