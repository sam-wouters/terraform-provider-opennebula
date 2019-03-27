package opennebula

import (
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
)

type UserVnets struct {
	UserVnet []*UserVnet `xml:"VNET"`
}

type UserVnet struct {
	Name        string        `xml:"NAME"`
	Id          int           `xml:"ID"`
	Uid         int           `xml:"UID"`
	Gid         int           `xml:"GID"`
	Uname       string        `xml:"UNAME"`
	Gname       string        `xml:"GNAME"`
	Permissions *Permissions  `xml:"PERMISSIONS"`
	Bridge      string        `xml:"BRIDGE"`
	ParentVnet  int           `xml:"PARENT_NETWORK_ID,omitempty"`
	Template    *VnetTemplate `xml:"TEMPLATE,omitempty"`
}

type VnetTemplate struct {
	Description     string `xml:"DESCRIPTION,omitempty"`
	Vn_Mad          string `xml:"VN_MAD,omitempty"`
	Phydev          string `xml:"PHYDEV,omitempty"`
	Vlan_id         int    `xml:"VLAN_ID,omitempty"`
	Security_Groups string `xml:"SECURITY_GROUPS,omitempty"`
	Dns             string `xml:"DNS,omitempty"`
	Gateway         string `xml:"GATEWAY,omitempty"`
	NetworkMask     string `xml:"NETWORK_MASK,omitempty"`
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
				Optional:    true,
				Computed:    true,
				Description: "ID of the user that will own the vnet",
			},
			"gid": {
				Type:        schema.TypeInt,
				Optional:    true,
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
			"vn_mad": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "VN driver to use. If empty, defaults to 'fw'",
			},
			"bridge": {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				Description:   "Name of the bridge interface to which the vnet should be associated",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"phydev": {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				Description:   "Name of the physical device to which the vlan should be associated",
				ConflictsWith: []string{"bridge", "reservation_vnet", "reservation_size"},
			},
			"vlan_id": {
				Type:          schema.TypeInt,
				Optional:      true,
				Description:   "ID of the vlan to be associated",
				ConflictsWith: []string{"bridge", "reservation_vnet", "reservation_size"},
			},
			"ip_start": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "Start IP of the range to be allocated",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"ip_size": {
				Type:          schema.TypeInt,
				Optional:      true,
				Description:   "Size (in number) of the ip range, defaults to 1 if empty",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"hold_size": {
				Type:          schema.TypeInt,
				Optional:      true,
				Description:   "Carve a network reservation of this size from the reservation starting from `ip-start`",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"reservation_vnet": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				Description:   "Create a reservation from this VNET ID",
				ConflictsWith: []string{"bridge", "ip_start", "ip_size", "hold_size"},
			},
			"reservation_size": {
				Type:          schema.TypeInt,
				Optional:      true,
				Description:   "Reserve this many IPs from reservation_vnet",
				ConflictsWith: []string{"bridge", "ip_start", "ip_size", "hold_size"},
			},
			"security_groups": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of Security Group IDs to be applied to the VNET",
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
			"dns": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "CONTEXT: Space separated list of dns IPs",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"gateway": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "CONTEXT: Gateway IP",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
			},
			"networkmask": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "CONTEXT: Network mask",
				ConflictsWith: []string{"reservation_vnet", "reservation_size"},
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

		if reservation_vnet <= 0 {
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

	} else { //New VNET
		var resp string
		var err error

		// build the vn template
		var vntmpl strings.Builder
		fmt.Fprintf(&vntmpl, "NAME=\"%s\"", d.Get("name").(string))
		if dscr, ok := d.GetOk("description"); ok {
			fmt.Fprintf(&vntmpl, "\nDESCRIPTION=\"%s\"", dscr.(string))
		}
		if br, ok := d.GetOk("bridge"); ok {
			fmt.Fprintf(&vntmpl, "\nBRIDGE=\"%s\"", br.(string))
		}
		if vnmad, ok := d.GetOk("vn_mad"); ok {
			fmt.Fprintf(&vntmpl, "\nVN_MAD=\"%s\"", d.Get("vn_mad").(string))
			if vnmad.(string) == "802.1q" {
				pdev, pdevok := d.GetOk("phydev")
				vlanid, vlanok := d.GetOk("vlan_id")
				if pdevok && vlanok {
					fmt.Fprintf(&vntmpl, "\nPHYDEV=\"%s\"", pdev.(string))
					fmt.Fprintf(&vntmpl, "\nVLAN_ID=\"%d\"", vlanid.(int))
				} else {
					return fmt.Errorf("For vn_mad 802.1q, both phydev and vlan_id should be given")
				}
			}
		}
		// CONTEXT params
		if nm, ok := d.GetOk("networkmask"); ok {
			fmt.Fprintf(&vntmpl, "\nNETWORK_MASK=\"%s\"", nm.(string))
		}
		if gw, ok := d.GetOk("gateway"); ok {
			fmt.Fprintf(&vntmpl, "\nGATEWAY=\"%s\"", gw.(string))
		}
		if dns, ok := d.GetOk("dns"); ok {
			fmt.Fprintf(&vntmpl, "\nDNS=\"%s\"", dns.(string))
		}
		resp, err = client.Call(
			"one.vn.allocate",
			vntmpl.String(),
			-1,
		)
		if err != nil {
			log.Printf(vntmpl.String())
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
		var size int
		if ar, ok := d.GetOk("ip_start"); ok {
			if as, ok := d.GetOk("ip_size"); ok {
				size = as.(int)
			} else {
				size = 1
			}
			_, a_err := client.Call(
				"one.vn.add_ar",
				intId(d.Id()),
				fmt.Sprintf(address_range_string, ar.(string), size),
			)
			if a_err != nil {
				return a_err
			}
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

	//Apply the security group rules if defined
	if security_groups, ok := d.GetOk("security_groups"); ok {
		err := setVnetSecurityGroups(client, intId(d.Id()), security_groups.([]interface{}))
		if err != nil {
			return err
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
	d.Set("vn_mad", vn.Template.Vn_Mad)
	d.Set("phydev", vn.Template.Phydev)
	d.Set("vlan_id", vn.Template.Vlan_id)
	d.Set("dns", vn.Template.Dns)
	d.Set("gateway", vn.Template.Gateway)
	d.Set("networkmask", vn.Template.NetworkMask)

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
	d.Partial(true)
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

	if d.HasChange("dns") {
		resp, err := client.Call(
			"one.vn.update",
			intId(d.Id()),
			fmt.Sprintf("DNS=\"%s\"", d.Get("dns").(string)),
			1,
		)
		if err != nil {
			return err
		}
		d.SetPartial("dns")
		log.Printf("[INFO] Successfully updated DNS for Vnet %s\n", resp)
	}

	if d.HasChange("gateway") {
		resp, err := client.Call(
			"one.vn.update",
			intId(d.Id()),
			fmt.Sprintf("GATEWAY=\"%s\"", d.Get("gateway").(string)),
			1,
		)
		if err != nil {
			return err
		}
		d.SetPartial("gateway")
		log.Printf("[INFO] Successfully updated GATEWAY for Vnet %s\n", resp)
	}

	if d.HasChange("networkmask") {
		resp, err := client.Call(
			"one.vn.update",
			intId(d.Id()),
			fmt.Sprintf("NETWORK_MASK=\"%s\"", d.Get("networkmask").(string)),
			1,
		)
		if err != nil {
			return err
		}
		d.SetPartial("networkmask")
		log.Printf("[INFO] Successfully updated NETWORK_MASK for Vnet %s\n", resp)
	}

	if d.HasChange("security_groups") {
		vnet_id, err := strconv.Atoi(d.Id())
		if err != nil {
			return nil
		}

		err = setVnetSecurityGroups(client, vnet_id, d.Get("security_groups").([]interface{}))
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

	var vn_ar_cmd string
	if d.HasChange("ip_start") {
		oldv, _ := d.GetChange("ip_start")
		if oldv.(string) == "" {
			// new address address_range_string
			vn_ar_cmd = "one.vn.add_ar"
		} else {
			log.Printf("[WARNING] Changing the IP address of the Vnet address range is currently not supported")
		}
	} else {
		vn_ar_cmd = "one.vn.update_ar"
	}

	if d.HasChange("ip_size") {
		var address_range_string = `AR = [
		AR_ID = 0,
		TYPE = IP4,
		IP = %s,
		SIZE = %d ]`
		resp, a_err := client.Call(
			vn_ar_cmd,
			intId(d.Id()),
			fmt.Sprintf(address_range_string, d.Get("ip_start").(string), d.Get("ip_size").(int)),
		)

		if a_err != nil {
			return a_err
		}
		d.SetPartial("ip_start")
		d.SetPartial("ip_size")
		log.Printf("[INFO] Successfully updated size of address range for Vnet %s\n", resp)
	}

	var change_own bool = false
	var newuid int = -1
	var newgid int = -1
	if d.HasChange("uid") && d.Get("uid") != "" {
		change_own = true
		newuid = d.Get("uid").(int)
	}
	if d.HasChange("gid") && d.Get("gid") != "" {
		change_own = true
		newgid = d.Get("gid").(int)
	}
	if change_own {
		resp, co_err := client.Call(
			"one.vn.chown",
			intId(d.Id()),
			newuid,
			newgid,
		)

		if co_err != nil {
			return co_err
		}
		d.SetPartial("uid")
		d.SetPartial("gid")
		log.Printf("[INFO] Successfully updated owner uid and gid for Vnet %s\n", resp)
	}

	if d.HasChange("permissions") && d.Get("permissions") != "" {
		resp, err := changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.vn.chmod")
		if err != nil {
			return err
		}
		log.Printf("[INFO] Successfully updated Vnet %s\n", resp)
	}

	d.Partial(false)
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
