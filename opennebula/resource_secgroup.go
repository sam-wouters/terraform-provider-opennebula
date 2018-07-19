package opennebula

import (
	"encoding/xml"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strings"
	"bytes"
	"github.com/fatih/structs"
	"strconv"
)


type SecurityGroups struct {
	XMLName         xml.Name        `xml:"SECURITY_GROUP_POOL"`
	SecurityGroup []*SecurityGroup  `xml:"SECURITY_GROUP"`
}

type SecurityGroup struct {
	XMLName         xml.Name     `xml:"SECURITY_GROUP"`
	Id              string       `xml:"ID"`
	Name            string       `xml:"NAME"`
	Uid             string       `xml:"UID"`
	Gid             string       `xml:"GID"`
	Uname           string       `xml:"UNAME"`
	Gname           string       `xml:"GNAME"`
	Permissions     *Permissions `xml:"PERMISSIONS"`
	SecurityGroupTemplate *SecurityGroupTemplate `xml:"TEMPLATE"`
}

type SecurityGroupTemplate struct {
	XMLName              xml.Name
	Name                 string                 `xml:"NAME"`
	Description          string                 `xml:"DESCRIPTION,omitempty"`
	SecurityGroupRules   []SecurityGroupRule    `xml:"RULE"`
}

type SecurityGroupRule struct {
	Protocol        string       `xml:"PROTOCOL"             structs:"protocol"`
	Range           string       `xml:"RANGE,omitempty"      structs:"range,omitempty"`
	RuleType        string       `xml:"RULE_TYPE"            structs:"rule_type,omitempty"`
	IP              string       `xml:"IP,omitempty"         structs:"ip,omitempty"`
	Size            string       `xml:"SIZE,omitempty"       structs:"size,omitempty"`
	NetworkId       string       `xml:"NETWORK_ID,omitempty" structs:"network_id,omitempty"`
	IcmpType        string       `xml:"ICMP_TYPE,omitempty"  structs:"icmp_type,omitempty"`
}


func resourceSecurityGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceSecurityGroupCreate,
		Read:   resourceSecurityGroupRead,
		Exists: resourceSecurityGroupExists,
		Update: resourceSecurityGroupUpdate,
		Delete: resourceSecurityGroupDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema {
			"name": {
				Type:			schema.TypeString,
				Required:		true,
				ForceNew:		true,
				Description:	"Name of the Security Group",

			},
			"description": {
				Type:			schema.TypeString,
				Optional:		true,
				Description:	"Description of the Security Group Rule Set",
			},
			"permissions": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Permissions for the Security Group (in Unix format, owner-group-other, use-manage-admin)",
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
				Type:			schema.TypeInt,
				Computed:		true,
				Description:	"ID of the user that will own the Security Group",
			},
			"gid": {
				Type:			schema.TypeInt,
				Computed:		true,
				Description:	"ID of the group that will own the Security Group",
			},
			"uname": {
				Type:			schema.TypeString,
				Computed:		true,
				Description:	"Name of the user that will own the Security Group",
			},
			"gname": {
				Type:			schema.TypeString,
				Computed:		true,
				Description:	"Name of the group that will own the Security Group",
			},
			"rule": {
				Type:			schema.TypeSet,
				Required:		true,
				MinItems:		1,
				Description:	"List of rules to be in the Security Group",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema {
						"protocol": {
							Type:			schema.TypeString,
							Description:	"Protocol for the rule, must be one of: ALL, TCP, UDP, ICMP or IPSEC",
							Required:		true,
							ValidateFunc: func (v interface{}, k string) (ws []string, errors []error) {
								validprotos := []string{"ALL", "TCP", "UDP", "ICMP", "IPSEC"}
								value := v.(string)

								if ! in_array(value, validprotos) {
									errors = append(errors, fmt.Errorf("Protocol %q must be one of: %s", k, strings.Join(validprotos,",")))
								}

								return
							},
						},
						"rule_type": {
							Type:			schema.TypeString,
							Description:	"Direction of the traffic flow to allow, must be INBOUND or OUTBOUND",
							Required:		true,
							ValidateFunc: func (v interface{}, k string) (ws []string, errors []error) {
								validtypes := []string{"INBOUND", "OUTBOUND"}
								value := v.(string)

								if ! in_array(value, validtypes) {
									errors = append(errors, fmt.Errorf("Rule type %q must be one of: %s", k, strings.Join(validtypes,",")))
								}

								return
							},

						},
						"ip": {
							Type:			schema.TypeString,
							Description: 	"IP (or starting IP if used with 'size') to apply the rule to",
							Optional:		true,
						},
						"size": {
							Type:			schema.TypeString,
							Description:	"Number of IPs to apply the rule from, starting with 'ip'",
							Optional:		true,
						},
						"range": {
							Type:			schema.TypeString,
							Description:	"Comma separated list of ports and port ranges",
							Optional:		true,
						},
						"icmp_type": {
							Type:			schema.TypeString,
							Description:	"Type of ICMP traffic to apply to when 'protocol' is ICMP",
							Optional:		true,
						},
						"network_id": {
							Type:			schema.TypeString,
							Description:	"VNET ID to be used as the source/destination IP addresses",
							Optional:		true,
						},
					},
				},
			},
			"commit": {
				Type:			schema.TypeBool,
				Description: 	"Should changes to the Security Group rules be commited to running Virtual Machines?",
				Optional: 		true,
				Default:    	true,
			},
		},
	}
}


func in_array(val string, array []string) (ok bool) {
    for i := range array {
        if ok = array[i] == val; ok {
            return
        }
    }
    return
}


func resourceSecurityGroupRead(d *schema.ResourceData, meta interface{}) error {
	var secgroup *SecurityGroup
	var secgroups *SecurityGroups

	client := meta.(*Client)
	found := false
	name := d.Get("name").(string)

	// Try to find the Security Group by ID, if specified
	if d.Id() != "" {
		resp, err := client.Call("one.secgroup.info", intId(d.Id()))
		if err == nil {
			found = true
			if err = xml.Unmarshal([]byte(resp), &secgroup); err != nil {
				return err
			}
		} else {
			log.Printf("Could not find Security Group by ID %s", d.Id())
		}
	}

	// Otherwise, try to find the vm by (user, name) as the de facto compound primary key
	if d.Id() == "" || !found {
		resp, err := client.Call("one.secgrouppool.info", -2, -1, -1)
		if err != nil {
			return err
		}

		if err = xml.Unmarshal([]byte(resp), &secgroups); err != nil {
			return err
		}

		for _, s := range secgroups.SecurityGroup {
			if s.Name == name {
				secgroup = s
				found = true
				break
			}
		}

		if !found || secgroup == nil {
			d.SetId("")
			log.Printf("Could not find Security Group with name %s for user %s", name, client.Username)
			return nil
		}
	}

	d.SetId(secgroup.Id)
	d.Set("uid", secgroup.Uid)
	d.Set("gid", secgroup.Gid)
	d.Set("uname", secgroup.Uname)
	d.Set("gname", secgroup.Gname)
	d.Set("permissions", permissionString(secgroup.Permissions))
	d.Set("description", secgroup.SecurityGroupTemplate.Description)

	if err := d.Set("rule", generateSecurityGroupMapFromStructs(secgroup.SecurityGroupTemplate.SecurityGroupRules)); err != nil {
		log.Printf("[WARN] Error setting rule for Security Group %s, error: %s", secgroup.Id, err)
	}

	return nil
}

func generateSecurityGroupMapFromStructs(slice []SecurityGroupRule) ([]map[string]interface{}){

	secrulemap := make([]map[string]interface{}, 0)

	for i := 0; i < len(slice); i++ {
		secrulemap = append(secrulemap, structs.Map(slice[i]))
	}

	return secrulemap
}

func resourceSecurityGroupExists(d *schema.ResourceData, meta interface{}) (bool, error) {
		err := resourceSecurityGroupRead(d, meta)
	// a terminated VM is in state 6 (DONE)
	if err != nil || d.Id() == "" {
		return false, err
	}

	return true, nil
}

func resourceSecurityGroupCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	var resp string
	var err error

	secgroupxml, xmlerr := generateSecurityGroupXML(d)
	if xmlerr != nil {
		return xmlerr	
	}

	resp, err = client.Call(
		"one.secgroup.allocate",
		secgroupxml,
	)

	if err != nil {
		return err
	}
	
	d.SetId(resp)

	return resourceSecurityGroupRead(d, meta)
}

func resourceSecurityGroupUpdate(d *schema.ResourceData, meta interface{}) error {

	// Enable partial state mode
	d.Partial(true)

	client := meta.(*Client)

	if d.HasChange("permissions") && d.Get("permissions") != "" {
		resp, err := changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.secgroup.chmod")
		if err != nil {
			return err
		}
		d.SetPartial("permissions")
		log.Printf("[INFO] Successfully updated Security Group %s\n", resp)
	}

	if d.HasChange("rule") && d.Get("rule") != "" {
		client := meta.(*Client)

		var resp string
		var err error

		secgroupxml, xmlerr := generateSecurityGroupXML(d)
		if xmlerr != nil {
			return xmlerr
		}

		objid,err := strconv.Atoi(d.Id())
		if err != nil {
			log.Printf("[ERROR] Unable to convert object id %s to integer", d.Id())
			return err
		}

		resp, err = client.Call(
			"one.secgroup.update",
			objid,
			secgroupxml,
			0,
		)

		if err != nil {
			return err
		}

		log.Printf("[INFO] Successfully updated Security Group template %s\n", resp)


		//Commit changes to running VMs if desired
		if d.Get("commit") == true {
			resp, err = client.Call(
				"one.secgroup.commit",
				objid,
				false, //Only update outdated VMs not all
			)

			if err != nil {
				return err
			}

			log.Printf("[INFO] Successfully commited Security Group %s changes to outdated Virtual Machines\n", resp)
		}

	}
	
	// We succeeded, disable partial mode. This causes Terraform to save
	// save all fields again.
	d.Partial(false)

	return nil
}

func resourceSecurityGroupDelete(d *schema.ResourceData, meta interface{}) error {
	err := resourceSecurityGroupRead(d, meta)
	if err != nil || d.Id() == "" {
		return err
	}

	client := meta.(*Client)
	resp, err := client.Call("one.secgroup.delete", intId(d.Id()))
	if err != nil {
		return err
	}

	log.Printf("[INFO] Successfully deleted Security Group %s\n", resp)
	return nil
}

func generateSecurityGroupXML(d *schema.ResourceData) (string, error) {

	//Generate rules definition
	rules := d.Get("rule").(*schema.Set).List()
	log.Printf("Number of Security Group rules: %d", len(rules))
	secgrouprules := make([]SecurityGroupRule, len(rules))

	for i := 0; i < len(rules); i++ {
		ruleconfig := rules[i].(map[string]interface{})

		var ruleprotocol string
		var ruletype string
		var ruleip string
		var rulesize string
		var rulerange string
		var ruleicmptype string
		var rulenetworkid string

		
		if ruleconfig["protocol"] != nil {
			ruleprotocol = ruleconfig["protocol"].(string)
		}

		if  ruleconfig["rule_type"] != nil {		
			ruletype = ruleconfig["rule_type"].(string)
		}

		if ruleconfig["ip"] != nil {				
			ruleip = ruleconfig["ip"].(string)
		}

		if ruleconfig["size"] != nil {		
			rulesize = ruleconfig["size"].(string)
		}

		if ruleconfig["range"] != nil {
			rulerange = ruleconfig["range"].(string)
		}

		if ruleconfig["icmp_type"] != nil {
			ruleicmptype = ruleconfig["icmp_type"].(string)
		}

		if ruleconfig["network_id"] != nil {
			rulenetworkid = ruleconfig["network_id"].(string)
		}

		secgrouprule := SecurityGroupRule {
			Protocol:		ruleprotocol,
			RuleType:		ruletype,
			IP:				ruleip,
			Size:			rulesize,
			Range:			rulerange,
			IcmpType:		ruleicmptype,
			NetworkId:		rulenetworkid,
		}

		secgrouprules[i] = secgrouprule
	}

	secgroupname := d.Get("name").(string)
	secgroupdescription := d.Get("description").(string)

	secgrouptpl := &SecurityGroupTemplate {
		Name:				secgroupname,
		Description: 		secgroupdescription,
		SecurityGroupRules: secgrouprules,
	}

	secgrouptpl.XMLName.Local = "SECURITY_GROUP"

	w := &bytes.Buffer{}

	//Encode the Security Group template schema to XML
	enc := xml.NewEncoder(w)
	//enc.Indent("", "  ")
	if err := enc.Encode(secgrouptpl); err != nil {
		return "", err
	}

	log.Printf("Security Group XML: %s", w.String())
	return w.String(), nil
}