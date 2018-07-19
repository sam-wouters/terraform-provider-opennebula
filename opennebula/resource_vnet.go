package opennebula

import (
	"encoding/xml"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"net"
	"strconv"
	"strings"
)

type UserVnets struct {
	UserVnet []*UserVnet `xml:"VNET"`
}

type UserVnet struct {
	Name        string       `xml:"NAME"`
	Id          int          `xml:"ID"`
	Uid         int          `xml:"UID"`
	Gid         int          `xml:"GID"`
	Uname       string       `xml:"UNAME"`
	Gname       string       `xml:"GNAME"`
	Permissions *Permissions `xml:"PERMISSIONS"`
	Bridge      string       `xml:"BRIDGE"`
	ParentVnet  int          `xml:"PARENT_NETWORK_ID,omitempty"`
	Template	*VnetTemplate	`xml:"TEMPLATE,omitempty"`
}

type VnetTemplate struct {
	Description      string  `xml:"DESCRIPTION,omitempty"`
	Security_Groups  string  `xml:"SECURITY_GROUPS,omitempty"`
}

func resourceVnet() *schema.Resource {
	return &schema.Resource{
		Create: resourceVnetCreate,
		Read:   resourceVnetRead,
		Exists: resourceVnetExists,
		Update: resourceVnetUpdate,
		Delete: resourceVnetDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the vnet",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the vnet, in OpenNebula's XML or String format",
			},
			"permissions": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Permissions for the vnet (in Unix format, owner-group-other, use-manage-admin)",
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					value := v.(string)

					if len(value) != 3 {
						errors = append(errors, fmt.Errorf("%q has specify 3 permission sets: owner-group-other", k))
					}

					all := true
					for _, c := range strings.Split(value, "") {
						if c < "0" || c > "7" {
							all = false
						}
					}
					if !all {
						errors = append(errors, fmt.Errorf("Each character in %q should specify a Unix-like permission set with a number from 0 to 7", k))
					}

					return
				},
			},

			"uid": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the user that will own the vnet",
			},
			"gid": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the group that will own the vnet",
			},
			"uname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the user that will own the vnet",
			},
			"gname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the group that will own the vnet",
			},
			"bridge": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Name of the bridge interface to which the vnet should be associated",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"ip_start": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Start IP of the range to be allocated",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"ip_size": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Size (in number) of the ip range",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"hold_size": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Carve a network reservation of this size from the reservation starting from `ip-start`",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"reservation_vnet": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Create a reservation from this VNET ID",
				ConflictsWith: []string{"bridge", "ip_start", "ip_size", "hold_size"},
			},
			"reservation_size": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Reserve this many IPs from reservation_vnet",
				ConflictsWith: []string{"bridge", "ip_start", "ip_size", "hold_size"},
			},
			"security_groups": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of Security Group IDs to be applied to the VNET",
				Elem: &schema.Schema {
					Type:	schema.TypeInt,
				},
			},
		},
	}
}

func resourceVnetCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	//VNET reservation
	if _, ok := d.GetOk("reservation_vnet"); ok {
		reservation_vnet := d.Get("reservation_vnet").(int)
		reservation_name := d.Get("name").(string)
		reservation_size := d.Get("reservation_size").(int)

		if reservation_vnet <= 0  {
			return fmt.Errorf("Reservation VNET ID must be greater than 0!")
		} else if reservation_size <= 0 {
			return fmt.Errorf("Reservation size must be greater than 0!")
		}

		//The API only takes ATTRIBUTE=VALUE for VNET reservations...
		reservation_string := "SIZE=%d\nNAME=\"%s\""

		resp, err := client.Call(
			"one.vn.reserve",
			reservation_vnet,
			fmt.Sprintf(reservation_string, reservation_size, reservation_name),
		)

		if err != nil {
			return err
		}

		d.SetId(resp)

		vnetid, err := strconv.Atoi(resp)
		if err != nil {
			return err
		}

		log.Printf("[DEBUG] New VNET reservation ID: %d", vnetid)

		//Apply the security group rules to the reservation if defined
		if security_groups, ok := d.GetOk("security_groups"); ok {
			err := setVnetSecurityGroups(client, vnetid, security_groups.([]interface{}))
			if err != nil {
				return err
			}
		}

	} else { //New VNET

		resp, err := client.Call(
			"one.vn.allocate",
			fmt.Sprintf("NAME = \"%s\"\n", d.Get("name").(string))+d.Get("description").(string)+"\nBRIDGE="+d.Get("bridge").(string),
			-1,
		)
		if err != nil {
			return err
		}

		d.SetId(resp)
		// update permisions
		if _, ok := d.GetOk("permissions"); ok {
			if _, err = changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.vn.chmod"); err != nil {
				return err
			}
		}

		// add address range and reservations
		var address_range_string = `AR = [
		  TYPE = IP4,
		  IP = %s,
		  SIZE = %d ]`
		_, a_err := client.Call(
			"one.vn.add_ar",
			intId(d.Id()),
			fmt.Sprintf(address_range_string, d.Get("ip_start").(string), d.Get("ip_size").(int)),
		)

		if a_err != nil {
			return a_err
		}

		if d.Get("hold_size").(int) > 0 {
			// add address range and reservations
			ip := net.ParseIP(d.Get("ip_start").(string))
			ip = ip.To4()

			for i := 0; i < d.Get("hold_size").(int); i++ {
				var address_reservation_string = `LEASES=[IP=%s]`
				_, r_err := client.Call(
					"one.vn.hold",
					intId(d.Id()),
					fmt.Sprintf(address_reservation_string, ip),
				)

				if r_err != nil {
					return r_err
				}

				ip[3]++
			}

		}
	}

	return resourceVnetRead(d, meta)
}

func setVnetSecurityGroups(client *Client, vnet_id int, security_group_ids []interface{}) error {

	//Convert the security group array to a comma separated string
	secgroup_list := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(security_group_ids)), ","), "[]")

	log.Printf("[DEBUG] Security group list: %s", secgroup_list)
	_, err := client.Call(
		"one.vn.update",
		vnet_id,
		fmt.Sprintf("SECURITY_GROUPS=\"%s\"", secgroup_list),
		1,
	)

	if err != nil {
		return err
	}

	return nil
}

func resourceVnetRead(d *schema.ResourceData, meta interface{}) error {
	var vn *UserVnet
	var vns *UserVnets

	client := meta.(*Client)
	found := false

	// Try to find the vnet by ID, if specified
	if d.Id() != "" {
		resp, err := client.Call("one.vn.info", intId(d.Id()), false)
		if err == nil {
			found = true
			if err = xml.Unmarshal([]byte(resp), &vn); err != nil {
				return err
			}
		} else {
			log.Printf("Could not find vnet by ID %s", d.Id())
		}
	}

	// Otherwise, try to find the vnet by (user, name) as the de facto compound primary key
	if d.Id() == "" || !found {
		resp, err := client.Call("one.vnpool.info", -2, -1, -1)
		if err != nil {
			return err
		}

		if err = xml.Unmarshal([]byte(resp), &vns); err != nil {
			return err
		}

		for _, t := range vns.UserVnet {
			if t.Name == d.Get("name").(string) {
				vn = t
				found = true
				break
			}
		}

		if !found || vn == nil {
			d.SetId("")
			log.Printf("Could not find vnet with name %s for user %s", d.Get("name").(string), client.Username)
			return nil
		}
	}

	d.SetId(strconv.Itoa(vn.Id))
	d.Set("name", vn.Name)
	d.Set("uid", vn.Uid)
	d.Set("gid", vn.Gid)
	d.Set("uname", vn.Uname)
	d.Set("gname", vn.Gname)
	d.Set("bridge", vn.Bridge)
	d.Set("reservation_vnet", vn.ParentVnet)
	d.Set("permissions", permissionString(vn.Permissions))

	secgroups_str := strings.Split(vn.Template.Security_Groups, ",")
	secgroups_int := []int{}

	for _, i := range secgroups_str {
		if i != "" {
			j, err := strconv.Atoi(i)
			if err != nil {
				return err
			}
			secgroups_int = append(secgroups_int, j)
		}
	}

	err := d.Set("security_groups", secgroups_int)
	if err != nil {
		log.Printf("[DEBUG] Error setting security groups on vnet: %s", err)
	}

	return nil
}

func resourceVnetExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	err := resourceVnetRead(d, meta)
	if err != nil || d.Id() == "" {
		return false, err
	}

	return true, nil
}

func resourceVnetUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	if d.HasChange("description") {
		_, err := client.Call(
			"one.vn.update",
			intId(d.Id()),
			fmt.Sprintf("DESCRIPTION=\"%s\"", d.Get("description").(string)),
			1,
		)
		if err != nil {
			return err
		}
	}

	if d.HasChange("security_groups") {
		vnet_id, err := strconv.Atoi(d.Id())
		if err != nil {
			return nil
		}


		err = setVnetSecurityGroups(client, vnet_id, d.Get("security_groups").([]interface {}))
		if err != nil {
			return err
		}
	}

	if d.HasChange("name") {
		resp, err := client.Call(
			"one.vn.rename",
			intId(d.Id()),
			d.Get("name").(string),
		)
		if err != nil {
			return err
		}
		log.Printf("[INFO] Successfully updated name for Vnet %s\n", resp)
	}

	if d.HasChange("ip_size") {
		var address_range_string = `AR = [
		AR_ID = 0,
		TYPE = IP4,
		IP = %s,
		SIZE = %d ]`
		resp, a_err := client.Call(
			"one.vn.update_ar",
			intId(d.Id()),
			fmt.Sprintf(address_range_string, d.Get("ip_start").(string), d.Get("ip_size").(int)),
		)

		if a_err != nil {
			return a_err
		}
		log.Printf("[INFO] Successfully updated size of address range for Vnet %s\n", resp)
	}

	if d.HasChange("ip_start") {
		log.Printf("[WARNING] Changing the IP address of the Vnet address range is currently not supported")
	}

	if d.HasChange("permissions") && d.Get("permissions") != "" {
		resp, err := changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.vn.chmod")
		if err != nil {
			return err
		}
		log.Printf("[INFO] Successfully updated Vnet %s\n", resp)
	}

	return nil
}

func resourceVnetDelete(d *schema.ResourceData, meta interface{}) error {
	err := resourceVnetRead(d, meta)
	if err != nil || d.Id() == "" {
		return err
	}

	client := meta.(*Client)
	if d.Get("hold_size").(int) > 0 {
		// add address range and reservations
		ip := net.ParseIP(d.Get("ip_start").(string))
		ip = ip.To4()

		for i := 0; i < d.Get("reservation_size").(int); i++ {
			var address_reservation_string = `LEASES=[IP=%s]`
			_, r_err := client.Call(
				"one.vn.release",
				intId(d.Id()),
				fmt.Sprintf(address_reservation_string, ip),
			)

			if r_err != nil {
				return r_err
			}

			ip[3]++
		}
		log.Printf("[INFO] Successfully released reservered IP addresses.")
	}

	resp, err := client.Call("one.vn.delete", intId(d.Id()), false)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Successfully deleted Vnet %s\n", resp)
	return nil
}
