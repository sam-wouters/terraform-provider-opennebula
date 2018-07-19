package opennebula

import (
	"github.com/hashicorp/terraform/helper/schema"
)

func dataVnet() *schema.Resource {
	return &schema.Resource{
		Read:   resourceVnetRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the vnet",
			},
		},
	}
}
