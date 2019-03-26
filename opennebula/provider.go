package opennebula

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The URL to your public or private OpenNebula",
				DefaultFunc: schema.EnvDefaultFunc("OPENNEBULA_ENDPOINT", nil),
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the user to identify as",
				DefaultFunc: schema.EnvDefaultFunc("OPENNEBULA_USERNAME", nil),
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The password for the user",
				DefaultFunc: schema.EnvDefaultFunc("OPENNEBULA_PASSWORD", nil),
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"opennebula_image": dataImage(),
			"opennebula_vnet":  dataVnet(),
			"opennebula_secgroup": dataSecurityGroup(),
			"opennebula_user": dataUser(),
			"opennebula_group": dataGroup(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"opennebula_template": resourceTemplate(),
			"opennebula_vnet":     resourceVnet(),
			"opennebula_vm":       resourceVm(),
			"opennebula_image":    resourceImage(),
			"opennebula_secgroup": resourceSecurityGroup(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	return NewClient(
		d.Get("endpoint").(string),
		d.Get("username").(string),
		d.Get("password").(string),
	)
}
