package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
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
