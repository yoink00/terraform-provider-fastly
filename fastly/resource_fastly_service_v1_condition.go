package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var conditionSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"statement": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The statement used to determine if the condition is met",
			},
			"priority": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     10,
				Description: "A number used to determine the order in which multiple conditions execute. Lower numbers execute first",
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Type of the condition, either `REQUEST`, `RESPONSE`, or `CACHE`",
				ValidateFunc: validateConditionType(),
			},
		},
	},
}

func flattenConditions(conditionList []*fastly.Condition) []map[string]interface{} {
	var cl []map[string]interface{}
	for _, c := range conditionList {
		// Convert Conditions to a map for saving to state.
		nc := map[string]interface{}{
			"name":      c.Name,
			"statement": c.Statement,
			"type":      c.Type,
			"priority":  c.Priority,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nc {
			if v == "" {
				delete(nc, k)
			}
		}

		cl = append(cl, nc)
	}

	return cl
}
