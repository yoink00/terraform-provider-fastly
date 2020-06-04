package fastly

import (
	"fmt"
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
	"strings"
)

var snippetSchema = &schema.Schema{
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
			"content": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The contents of the VCL snippet",
			},
			"priority": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "Determines ordering for multiple snippets. Lower priorities execute first. (Default: 100)",
			},
		},
	},
}

func buildSnippet(snippetMap interface{}) (*fastly.CreateSnippetInput, error) {
	df := snippetMap.(map[string]interface{})
	opts := fastly.CreateSnippetInput{
		Name:     df["name"].(string),
		Content:  df["content"].(string),
		Priority: df["priority"].(int),
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

type SnippetAttributeHandler struct {
	*DefaultAttributeHandler
}

func NewSnippet() AttributeHandler {
	return &SnippetAttributeHandler{
		&DefaultAttributeHandler{
			schema: vclSchema,
			key:    "snippet",
		},
	}
}

func flattenSnippets(snippetList []*fastly.Snippet) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, snippet := range snippetList {
		// Skip dynamic snippets
		if snippet.Dynamic == 1 {
			continue
		}

		// Convert VCLs to a map for saving to state.
		snippetMap := map[string]interface{}{
			"name":     snippet.Name,
			"type":     snippet.Type,
			"priority": int(snippet.Priority),
			"content":  snippet.Content,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range snippetMap {
			if v == "" {
				delete(snippetMap, k)
			}
		}

		sl = append(sl, snippetMap)
	}

	return sl
}

func (h *SnippetAttributeHandler) Process(d *schema.ResourceData, latestVersion int, conn *fastly.Client) error {
	// Note: as above with Gzip and S3 logging, we don't utilize the PUT
	// endpoint to update a VCL snippet, we simply destroy it and create a new one.
	oldSnippetVal, newSnippetVal := d.GetChange("snippet")
	if oldSnippetVal == nil {
		oldSnippetVal = new(schema.Set)
	}
	if newSnippetVal == nil {
		newSnippetVal = new(schema.Set)
	}

	oldSnippetSet := oldSnippetVal.(*schema.Set)
	newSnippetSet := newSnippetVal.(*schema.Set)

	remove := oldSnippetSet.Difference(newSnippetSet).List()
	add := newSnippetSet.Difference(oldSnippetSet).List()

	// Delete removed VCL Snippet configurations
	for _, dRaw := range remove {
		df := dRaw.(map[string]interface{})
		opts := fastly.DeleteSnippetInput{
			Service: d.Id(),
			Version: latestVersion,
			Name:    df["name"].(string),
		}

		log.Printf("[DEBUG] Fastly VCL Snippet Removal opts: %#v", opts)
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
		opts, err := buildSnippet(dRaw.(map[string]interface{}))
		if err != nil {
			log.Printf("[DEBUG] Error building VCL Snippet: %s", err)
			return err
		}
		opts.Service = d.Id()
		opts.Version = latestVersion

		log.Printf("[DEBUG] Fastly VCL Snippet Addition opts: %#v", opts)
		_, err = conn.CreateSnippet(opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *SnippetAttributeHandler) Read(d *schema.ResourceData, conn *fastly.Client, s *fastly.ServiceDetail) error {
	// refresh VCL Snippets
	log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
	snippetList, err := conn.ListSnippets(&fastly.ListSnippetsInput{
		Service: d.Id(),
		Version: s.ActiveVersion.Number,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	vsl := flattenSnippets(snippetList)

	if err := d.Set("snippet", vsl); err != nil {
		log.Printf("[WARN] Error setting VCL Snippets for (%s): %s", d.Id(), err)
	}
	return nil
}
