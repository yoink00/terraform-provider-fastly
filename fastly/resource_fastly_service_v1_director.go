package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var directorSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A name to refer to this director",
			},
			"backends": {
				Type:        schema.TypeSet,
				Required:    true,
				Description: "List of backends associated with this director",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			// optional fields
			"capacity": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "Load balancing weight for the backends",
			},
			"comment": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"shield": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Selected POP to serve as a 'shield' for origin servers.",
			},
			"quorum": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      75,
				Description:  "Percentage of capacity that needs to be up for the director itself to be considered up",
				ValidateFunc: validateDirectorQuorum(),
			},
			"type": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1,
				Description:  "Type of load balance group to use. Integer, 1 to 4. Values: 1 (random), 3 (hash), 4 (client)",
				ValidateFunc: validateDirectorType(),
			},
			"retries": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     5,
				Description: "How many backends to search if it fails",
			},
		},
	},
}

func flattenDirectors(directorList []*fastly.Director, directorBackendList []*fastly.DirectorBackend) []map[string]interface{} {
	var dl []map[string]interface{}
	for _, d := range directorList {
		// Convert Director to a map for saving to state.
		nd := map[string]interface{}{
			"name":     d.Name,
			"comment":  d.Comment,
			"shield":   d.Shield,
			"type":     d.Type,
			"quorum":   int(d.Quorum),
			"capacity": int(d.Capacity),
			"retries":  int(d.Retries),
		}

		var b []interface{}
		for _, db := range directorBackendList {
			if d.Name == db.Director {
				b = append(b, db.Backend)
			}
		}
		if len(b) > 0 {
			nd["backends"] = schema.NewSet(schema.HashString, b)
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nd {
			if v == "" {
				delete(nd, k)
			}
		}

		dl = append(dl, nd)
	}
	return dl
}
