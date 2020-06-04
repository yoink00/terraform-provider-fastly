package fastly

import (
	"fmt"
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
)

var aclSchema = &schema.Schema{
	Type:     schema.TypeSet,
	Optional: true,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{
			// Required fields
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name to refer to this ACL",
			},
			// Optional fields
			"acl_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Generated acl id",
			},
		},
	},
}

type ACLAttributeHandler struct {
	*DefaultAttributeHandler
}

func NewACL() AttributeHandler {
	return &ACLAttributeHandler{
		&DefaultAttributeHandler{
			schema: aclSchema,
			key:    "acl",
		},
	}
}

func flattenAclEntries(aclEntryList []*fastly.ACLEntry) []map[string]interface{} {

	var resultList []map[string]interface{}

	for _, currentAclEntry := range aclEntryList {
		aes := map[string]interface{}{
			"id":      currentAclEntry.ID,
			"ip":      currentAclEntry.IP,
			"subnet":  currentAclEntry.Subnet,
			"negated": currentAclEntry.Negated,
			"comment": currentAclEntry.Comment,
		}

		for k, v := range aes {
			if v == "" {
				delete(aes, k)
			}
		}

		resultList = append(resultList, aes)
	}

	return resultList
}

func flattenACLs(aclList []*fastly.ACL) []map[string]interface{} {
	var al []map[string]interface{}
	for _, acl := range aclList {
		// Convert VCLs to a map for saving to state.
		vclMap := map[string]interface{}{
			"acl_id": acl.ID,
			"name":   acl.Name,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range vclMap {
			if v == "" {
				delete(vclMap, k)
			}
		}

		al = append(al, vclMap)
	}

	return al
}

func (h *ACLAttributeHandler) Process(d *schema.ResourceData, latestVersion int, conn *fastly.Client) error {
	oldACLVal, newACLVal := d.GetChange("acl")
	if oldACLVal == nil {
		oldACLVal = new(schema.Set)
	}
	if newACLVal == nil {
		newACLVal = new(schema.Set)
	}

	oldACLSet := oldACLVal.(*schema.Set)
	newACLSet := newACLVal.(*schema.Set)

	remove := oldACLSet.Difference(newACLSet).List()
	add := newACLSet.Difference(oldACLSet).List()

	// Delete removed ACL configurations
	for _, vRaw := range remove {
		val := vRaw.(map[string]interface{})
		opts := fastly.DeleteACLInput{
			Service: d.Id(),
			Version: latestVersion,
			Name:    val["name"].(string),
		}

		log.Printf("[DEBUG] Fastly ACL removal opts: %#v", opts)
		err := conn.DeleteACL(&opts)

		if errRes, ok := err.(*fastly.HTTPError); ok {
			if errRes.StatusCode != 404 {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	// POST new ACL configurations
	for _, vRaw := range add {
		val := vRaw.(map[string]interface{})
		opts := fastly.CreateACLInput{
			Service: d.Id(),
			Version: latestVersion,
			Name:    val["name"].(string),
		}

		log.Printf("[DEBUG] Fastly ACL creation opts: %#v", opts)
		_, err := conn.CreateACL(&opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *ACLAttributeHandler) Read(d *schema.ResourceData, conn *fastly.Client, s *fastly.ServiceDetail) error {
	// refresh ACLs
	log.Printf("[DEBUG] Refreshing ACLs for (%s)", d.Id())
	aclList, err := conn.ListACLs(&fastly.ListACLsInput{
		Service: d.Id(),
		Version: s.ActiveVersion.Number,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up ACLs for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	al := flattenACLs(aclList)

	if err := d.Set("acl", al); err != nil {
		log.Printf("[WARN] Error setting ACLs for (%s): %s", d.Id(), err)
	}
	return nil
}
