package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
)

var dictionarySchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			// Required fields
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name to refer to this Dictionary",
			},
			// Optional fields
			"dictionary_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Generated dictionary ID",
			},
			"write_only": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Determines if items in the dictionary are readable or not",
			},
		},
	},
}

func buildDictionary(dictMap interface{}) (*fastly.CreateDictionaryInput, error) {
	df := dictMap.(map[string]interface{})
	opts := fastly.CreateDictionaryInput{
		Name:      df["name"].(string),
		WriteOnly: fastly.CBool(df["write_only"].(bool)),
	}

	return &opts, nil
}

func flattenDictionaries(dictList []*fastly.Dictionary) []map[string]interface{} {
	var dl []map[string]interface{}
	for _, currentDict := range dictList {

		dictMapString := map[string]interface{}{
			"dictionary_id": currentDict.ID,
			"name":          currentDict.Name,
			"write_only":    currentDict.WriteOnly,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range dictMapString {
			if v == "" {
				delete(dictMapString, k)
			}
		}

		dl = append(dl, dictMapString)
	}

	return dl
}

func processDictionary(d *schema.ResourceData, latestVersion int, conn *fastly.Client) (error, bool) {
	oldDictVal, newDictVal := d.GetChange("dictionary")

	if oldDictVal == nil {
		oldDictVal = new(schema.Set)
	}
	if newDictVal == nil {
		newDictVal = new(schema.Set)
	}

	oldDictSet := oldDictVal.(*schema.Set)
	newDictSet := newDictVal.(*schema.Set)

	remove := oldDictSet.Difference(newDictSet).List()
	add := newDictSet.Difference(oldDictSet).List()

	// Delete removed dictionary configurations
	for _, dRaw := range remove {
		df := dRaw.(map[string]interface{})
		opts := fastly.DeleteDictionaryInput{
			Service: d.Id(),
			Version: latestVersion,
			Name:    df["name"].(string),
		}

		log.Printf("[DEBUG] Fastly Dictionary Removal opts: %#v", opts)
		err := conn.DeleteDictionary(&opts)
		if errRes, ok := err.(*fastly.HTTPError); ok {
			if errRes.StatusCode != 404 {
				return err, true
			}
		} else if err != nil {
			return err, true
		}
	}

	// POST new dictionary configurations
	for _, dRaw := range add {
		opts, err := buildDictionary(dRaw.(map[string]interface{}))
		if err != nil {
			log.Printf("[DEBUG] Error building Dicitionary: %s", err)
			return err, true
		}
		opts.Service = d.Id()
		opts.Version = latestVersion

		log.Printf("[DEBUG] Fastly Dictionary Addition opts: %#v", opts)
		_, err = conn.CreateDictionary(opts)
		if err != nil {
			return err, true
		}
	}
	return nil, false
}
