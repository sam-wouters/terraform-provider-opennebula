package opennebula

import (
	"github.com/hashicorp/terraform/helper/schema"
)

func dataSecurityGroup() *schema.Resource {
	return &schema.Resource{
		Read:   resourceSecurityGroupRead,

		Schema: map[string]*schema.Schema {
			"name": {
				Type:			schema.TypeString,
				Required:		true,
				ForceNew:		true,
				Description:	"Name of the Security Group",
			},
		},
	}
}

