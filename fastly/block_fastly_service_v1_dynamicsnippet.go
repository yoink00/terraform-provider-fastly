package fastly

import (
	"fmt"
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
	"strings"
)

var dynamicsnippetSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A unique name to refer to this VCL snippet",
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "One of init, recv, hit, miss, pass, fetch, error, deliver, log, none",
				ValidateFunc: validateSnippetType(),
			},
			"priority": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "Determines ordering for multiple snippets. Lower priorities execute first. (Default: 100)",
			},
			"snippet_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Generated VCL snippet Id",
			},
		},
	},
}

type DynamicSnippetAttributeHandler struct {
	*DefaultAttributeHandler
}

func NewDynamicSnippet() AttributeHandler {
	return &DynamicSnippetAttributeHandler{
		&DefaultAttributeHandler{
			schema: dynamicsnippetSchema,
			key:    "dynamicsnippet",
		},
	}
}

func buildDynamicSnippet(dynamicSnippetMap interface{}) (*fastly.CreateSnippetInput, error) {
	df := dynamicSnippetMap.(map[string]interface{})
	opts := fastly.CreateSnippetInput{
		Name:     df["name"].(string),
		Priority: df["priority"].(int),
		Dynamic:  1,
	}

	snippetType := strings.ToLower(df["type"].(string))
	switch snippetType {
	case "init":
		opts.Type = fastly.SnippetTypeInit
	case "recv":
		opts.Type = fastly.SnippetTypeRecv
	case "hash":
		opts.Type = fastly.SnippetTypeHash
	case "hit":
		opts.Type = fastly.SnippetTypeHit
	case "miss":
		opts.Type = fastly.SnippetTypeMiss
	case "pass":
		opts.Type = fastly.SnippetTypePass
	case "fetch":
		opts.Type = fastly.SnippetTypeFetch
	case "error":
		opts.Type = fastly.SnippetTypeError
	case "deliver":
		opts.Type = fastly.SnippetTypeDeliver
	case "log":
		opts.Type = fastly.SnippetTypeLog
	case "none":
		opts.Type = fastly.SnippetTypeNone
	}

	return &opts, nil
}

func flattenDynamicSnippets(dynamicSnippetList []*fastly.Snippet) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, dynamicSnippet := range dynamicSnippetList {
		// Skip non-dynamic snippets
		if dynamicSnippet.Dynamic == 0 {
			continue
		}

		// Convert VCLs to a map for saving to state.
		dynamicSnippetMap := map[string]interface{}{
			"snippet_id": dynamicSnippet.ID,
			"name":       dynamicSnippet.Name,
			"type":       dynamicSnippet.Type,
			"priority":   int(dynamicSnippet.Priority),
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range dynamicSnippetMap {
			if v == "" {
				delete(dynamicSnippetMap, k)
			}
		}

		sl = append(sl, dynamicSnippetMap)
	}

	return sl
}

func (h *DynamicSnippetAttributeHandler) Process(d *schema.ResourceData, latestVersion int, conn *fastly.Client) error {
	// Note: as above with Gzip and S3 logging, we don't utilize the PUT
	// endpoint to update a VCL dynamic snippet, we simply destroy it and create a new one.
	oldDynamicSnippetVal, newDynamicSnippetVal := d.GetChange("dynamicsnippet")
	if oldDynamicSnippetVal == nil {
		oldDynamicSnippetVal = new(schema.Set)
	}
	if newDynamicSnippetVal == nil {
		newDynamicSnippetVal = new(schema.Set)
	}

	oldDynamicSnippetSet := oldDynamicSnippetVal.(*schema.Set)
	newDynamicSnippetSet := newDynamicSnippetVal.(*schema.Set)

	remove := oldDynamicSnippetSet.Difference(newDynamicSnippetSet).List()
	add := newDynamicSnippetSet.Difference(oldDynamicSnippetSet).List()

	// Delete removed VCL Snippet configurations
	for _, dRaw := range remove {
		df := dRaw.(map[string]interface{})
		opts := fastly.DeleteSnippetInput{
			Service: d.Id(),
			Version: latestVersion,
			Name:    df["name"].(string),
		}

		log.Printf("[DEBUG] Fastly VCL Dynamic Snippet Removal opts: %#v", opts)
		err := conn.DeleteSnippet(&opts)
		if errRes, ok := err.(*fastly.HTTPError); ok {
			if errRes.StatusCode != 404 {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	// POST new VCL Snippet configurations
	for _, dRaw := range add {
		opts, err := buildDynamicSnippet(dRaw.(map[string]interface{}))
		if err != nil {
			log.Printf("[DEBUG] Error building VCL Dynamic Snippet: %s", err)
			return err
		}
		opts.Service = d.Id()
		opts.Version = latestVersion

		log.Printf("[DEBUG] Fastly VCL Dynamic Snippet Addition opts: %#v", opts)
		_, err = conn.CreateSnippet(opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *DynamicSnippetAttributeHandler) Read(d *schema.ResourceData, conn *fastly.Client, s *fastly.ServiceDetail) error {
	log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
	snippetList, err := conn.ListSnippets(&fastly.ListSnippetsInput{
		Service: d.Id(),
		Version: s.ActiveVersion.Number,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	dynamicSnippets := flattenDynamicSnippets(snippetList)

	if err := d.Set("dynamicsnippet", dynamicSnippets); err != nil {
		log.Printf("[WARN] Error setting VCL Dynamic Snippets for (%s): %s", d.Id(), err)
	}
	return nil
}
