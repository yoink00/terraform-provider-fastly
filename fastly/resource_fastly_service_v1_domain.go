package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var domainSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Required: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The domain that this Service will respond to",
			},

			"comment": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	},
}

func flattenDomains(list []*fastly.Domain) []map[string]interface{} {
	dl := make([]map[string]interface{}, 0, len(list))

	for _, d := range list {
		dl = append(dl, map[string]interface{}{
			"name":    d.Name,
			"comment": d.Comment,
		})
	}

	return dl
}
