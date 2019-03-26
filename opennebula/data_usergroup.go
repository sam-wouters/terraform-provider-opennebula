package opennebula

import (
	"github.com/hashicorp/terraform/helper/schema"
)

func dataUser() *schema.Resource {
	return &schema.Resource{
		Read: resourceUserRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the User",
			},
		},
	}
}

func dataGroup() *schema.Resource {
	return &schema.Resource{
		Read: resourceGroupRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Group",
			},
		},
	}
}
