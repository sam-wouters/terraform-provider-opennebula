package opennebula

import (
	"github.com/hashicorp/terraform/helper/schema"
)

func dataImage() *schema.Resource {
	return &schema.Resource{
		Read:   resourceImageRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:			schema.TypeString,
				Required:		true,
				Description:	"Name of the Image",
			},
		},
	}
}
