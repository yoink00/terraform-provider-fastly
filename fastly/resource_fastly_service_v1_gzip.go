package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
	"strings"
)

var gzipSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			// required fields
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A name to refer to this gzip condition",
			},
			// optional fields
			"content_types": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Content types to apply automatic gzip to",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"extensions": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "File extensions to apply automatic gzip to. Do not include '.'",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"cache_condition": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Name of a condition controlling when this gzip configuration applies.",
			},
		},
	},
}

func flattenGzips(gzipsList []*fastly.Gzip) []map[string]interface{} {
	var gl []map[string]interface{}
	for _, g := range gzipsList {
		// Convert Gzip to a map for saving to state.
		ng := map[string]interface{}{
			"name":            g.Name,
			"cache_condition": g.CacheCondition,
		}

		if g.Extensions != "" {
			e := strings.Split(g.Extensions, " ")
			var et []interface{}
			for _, ev := range e {
				et = append(et, ev)
			}
			ng["extensions"] = schema.NewSet(schema.HashString, et)
		}

		if g.ContentTypes != "" {
			c := strings.Split(g.ContentTypes, " ")
			var ct []interface{}
			for _, cv := range c {
				ct = append(ct, cv)
			}
			ng["content_types"] = schema.NewSet(schema.HashString, ct)
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range ng {
			if v == "" {
				delete(ng, k)
			}
		}

		gl = append(gl, ng)
	}

	return gl
}
