package opennebula

import (
	"encoding/xml"
	"fmt"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strings"
	"time"
	"bytes"
	"io"
)

type UserVm struct {
	Id              string       `xml:"ID"`
	Name            string       `xml:"NAME"`
	Uid             int          `xml:"UID"`
	Gid             int          `xml:"GID"`
	Uname           string       `xml:"UNAME"`
	Gname           string       `xml:"GNAME"`
	Permissions     *Permissions `xml:"PERMISSIONS"`
	State           int          `xml:"STATE"`
	LcmState        int          `xml:"LCM_STATE"`
	VmTemplate      *VmTemplate  `xml:"TEMPLATE"`
	VmUserTemplate  StringMap    `xml:"USER_TEMPLATE"`
}

type UserVms struct {
	UserVm []*UserVm `xml:"VM"`
}

type VmTemplate struct {
	//Context *Context `xml:"CONTEXT"`
	XMLName     xml.Name               `xml:"TEMPLATE"`
	Name        string                 `xml:"NAME,omitempty"`
	VCPU        int                    `xml:"VCPU"`
	CPU         float64                `xml:"CPU"`
	Memory      int                    `xml:"MEMORY"`
	ContextVars StringMap              `xml:"CONTEXT"`
	NICs        []VirtualMachineNIC    `xml:"NIC"`
	Disks       []VirtualMachineDisk   `xml:"DISK"`
	Graphics    VirtualMachineGraphics `xml:"GRAPHICS"`
	OS          VirtualMachineOS       `xml:"OS"`
	RAW         VirtualMachineRAW      `xml:"RAW"`
}

type VirtualMachineNIC struct {
	XMLName          xml.Name    `xml:"NIC"`
	NIC_ID           int         `xml:"NIC_ID,omitempty"`
	IP               string      `xml:"IP,omitempty"`
	Model            string      `xml:"MODEL,omitempty"`
	MAC              string      `xml:"MAC,omitempty"`
	Network_ID       int         `xml:"NETWORK_ID"`
	Security_Groups  string      `xml:"SECURITY_GROUPS,omitempty"`
}

type VirtualMachineDisk struct {
	XMLName       xml.Name    `xml:"DISK"`
	Disk_ID       string      `xml:"DISK_ID,omitempty"`
	Image_ID      int         `xml:"IMAGE_ID"`
	Size          int         `xml:"SIZE,omitempty"`
	Target        string      `xml:"TARGET,omitempty"`
	Driver        string      `xml:"DRIVER,omitempty"`
}

type VirtualMachineGraphics struct {
	Listen       string      `xml:"LISTEN,omitempty"`
	Type         string      `xml:"TYPE,omitempty"`
}

type VirtualMachineOS struct {
	Arch       string        `xml:"ARCH,omitempty"`
	Boot       string        `xml:"BOOT,omitempty"`
}

type VirtualMachineRAW struct {
	Type       string        `xml:"TYPE,omitempty"`
	Data       string        `xml:"DATA,omitempty"`
}


//This type and the MarshalXML functions are needed to handle converting the CONTEXT map to xml and back
//From: https://stackoverflow.com/questions/30928770/marshall-map-to-xml-in-go/33110881
type StringMap map[string]string
type xmlMapEntry struct {
    XMLName xml.Name
    Value   string `xml:",chardata"`
}
// MarshalXML marshals the map to XML, with each key in the map being a
// tag and it's corresponding value being it's contents.
func (m StringMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
    if len(m) == 0 {
        return nil
    }

    err := e.EncodeToken(start)
    if err != nil {
        return err
    }

    for k, v := range m {
        e.Encode(xmlMapEntry{XMLName: xml.Name{Local: k}, Value: v})
    }

    return e.EncodeToken(start.End())
}

// UnmarshalXML unmarshals the XML into a map of string to strings,
// creating a key in the map for each tag and setting it's value to the
// tags contents.
//
// The fact this function is on the pointer of Map is important, so that
// if m is nil it can be initialized, which is often the case if m is
// nested in another xml structurel. This is also why the first thing done
// on the first line is initialize it.
func (m *StringMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
    *m = StringMap{}
    for {
        var e xmlMapEntry

        err := d.Decode(&e)
        if err == io.EOF {
            break
        } else if err != nil {
            return err
        }

        (*m)[e.XMLName.Local] = e.Value
    }
    return nil
}


func resourceVm() *schema.Resource {
	return &schema.Resource{
		Create: resourceVmCreate,
		Read:   resourceVmRead,
		Exists: resourceVmExists,
		Update: resourceVmUpdate,
		Delete: resourceVmDelete,
		CustomizeDiff: resourceVMCustomizeDiff,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Name of the VM. If empty, defaults to 'templatename-<vmid>'",
			},
			"instance": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Final name of the VM instance",
			},
			"template_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Id of the VM template to use. Either 'template_name' or 'template_id' is required",
				ConflictsWith: []string{"disk", "graphics", "nic", "context", "os"},
			},
			"permissions": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Permissions for the template (in Unix format, owner-group-other, use-manage-admin)",
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
				Description: "ID of the user that will own the VM",
			},
			"gid": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the group that will own the VM",
			},
			"uname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the user that will own the VM",
			},
			"gname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the group that will own the VM",
			},
			"state": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Current state of the VM",
			},
			"lcmstate": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Current LCM state of the VM",
			},
			"cpu": {
				Type:        schema.TypeFloat,
				Required:    true,
				ForceNew:    true,
				Description: "Amount of CPU quota assigned to the virtual machine",
			},
			"vcpu": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Number of virtual CPUs assigned to the virtual machine",
			},
			"memory": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Amount of memory (RAM) in MB assigned to the virtual machine",
			},
			"context": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Description: "Context variables",
			},
			"disk": {
				Type:        schema.TypeSet,
				Optional:    true,
				//Computed:    true,
				MinItems:    1,
				MaxItems:    8,
				ConflictsWith: []string{"template_id"},
				ForceNew:    true,
				Description: "Definition of disks assigned to the Virtual Machine",
				Elem: &schema.Resource {
					Schema: map[string]*schema.Schema {
						"image_id": {
							Type:     schema.TypeInt,
							Required: true,
							ForceNew: true,
						},
						"size": {
							Type:     schema.TypeInt,
							Optional: true,
							ForceNew: true,
						},
						"target": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"driver": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
			"graphics": {
				Type:        schema.TypeSet,
				Optional:    true,
				//Computed:    true,
				MinItems:    1,
				MaxItems:    1,
				ConflictsWith: []string{"template_id"},
				ForceNew:    true,
				Description: "Definition of graphics adapter assigned to the Virtual Machine",
				Elem: &schema.Resource {
					Schema: map[string]*schema.Schema {
						"listen": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
			},
			"nic": {
				Type:        schema.TypeSet,
				Optional:    true,
				//Computed:    true,
				MinItems:    1,
				MaxItems:    8,
				ConflictsWith: []string{"template_id"},
				ForceNew:    true,
				Description: "Definition of network adapter(s) assigned to the Virtual Machine",
				Elem: &schema.Resource {
					Schema: map[string]*schema.Schema {
						"ip": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"mac": {
							Type:     schema.TypeString,
							Computed: true,
							ForceNew: true,
						},
						"model": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"network_id": {
							Type:     schema.TypeInt,
							Required: true,
							ForceNew: true,
						},
						"nic_id": {
							Type:     schema.TypeInt,
							Computed: true,
							ForceNew: true,
						},
						"security_groups": {
							Type:     schema.TypeList,
							Optional: true,
							ForceNew: true,
							Elem: &schema.Schema {
								Type:	schema.TypeInt,
							},
						},
					},
				},
				Set: resourceVMNicHash,
			},
			"os": {
				Type:        schema.TypeSet,
				Optional:    true,
				//Computed:    true,
				MinItems:    1,
				MaxItems:    1,
				ConflictsWith: []string{"template_id"},
				ForceNew:    true,
				Description: "Definition of OS boot and type for the Virtual Machine",
				Elem: &schema.Resource {
					Schema: map[string]*schema.Schema {
						"arch": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"boot": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
			},
			"raw": {
				Type:        schema.TypeSet,
				Optional:    true,
				//Computed:    true,
				MinItems:    0,
				MaxItems:    1,
				ConflictsWith: []string{"template_id"},
				ForceNew:    true,
				Description: "Definition of RAW parameters for the Virtual Machine",
				Elem: &schema.Resource {
					Schema: map[string]*schema.Schema {
						"data": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
			},
			"ip": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Primary IP address assigned by OpenNebula",
			},
		},
	}
}

func resourceVmCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	//Call one.template.instantiate only if template_id is defined
	//otherwise use one.vm.allocate
	var resp string
	var err error
	if v, ok := d.GetOk("template_id"); ok {
		resp, err = client.Call(
			"one.template.instantiate",
			v,
			d.Get("name"),
			false,
			"",
			false,
		)

	} else {
		vmxml, xmlerr := generateVmXML(d)
		if xmlerr != nil {
			return xmlerr
		}

		resp, err = client.Call(
			"one.vm.allocate",
			vmxml,
			false,
		)
	}

	if err != nil {
		return err
	}

	d.SetId(resp)

	_, err = waitForVmState(d, meta, "running")
	if err != nil {
		return fmt.Errorf(
			"Error waiting for virtual machine (%s) to be in state RUNNING: %s", d.Id(), err)
	}

	//Set the permissions on the VM if it was defined, otherwise use the UMASK in OpenNebula
	if _, ok := d.GetOk("permissions"); ok {
		if _, err = changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.vm.chmod"); err != nil {
			return err
		}
	}

	return resourceVmRead(d, meta)
}

func resourceVmRead(d *schema.ResourceData, meta interface{}) error {
	var vm *UserVm
	var vms *UserVms

	client := meta.(*Client)
	found := false
	name := d.Get("name").(string)
	if name == "" {
		name = d.Get("instance").(string)
	}

	// Try to find the vm by ID, if specified
	if d.Id() != "" {
		resp, err := client.Call("one.vm.info", intId(d.Id()))
		if err == nil {
			found = true
			if err = xml.Unmarshal([]byte(resp), &vm); err != nil {
				return err
			}
		} else {
			log.Printf("Could not find VM by ID %s", d.Id())
		}
	}

	// Otherwise, try to find the vm by (user, name) as the de facto compound primary key
	if d.Id() == "" || !found {
		resp, err := client.Call("one.vmpool.info", -3, -1, -1)
		if err != nil {
			return err
		}

		if err = xml.Unmarshal([]byte(resp), &vms); err != nil {
			return err
		}

		for _, v := range vms.UserVm {
			if v.Name == name {
				vm = v
				found = true
				break
			}
		}

		if !found || vm == nil {
			d.SetId("")
			log.Printf("Could not find vm with name %s for user %s", name, client.Username)
			return nil
		}
	}

	d.SetId(vm.Id)
	d.Set("instance", vm.Name)
	d.Set("uid", vm.Uid)
	d.Set("gid", vm.Gid)
	d.Set("uname", vm.Uname)
	d.Set("gname", vm.Gname)
	d.Set("state", vm.State)
	d.Set("lcmstate", vm.LcmState)
	//TODO fix this:
	//d.Set("ip", vm.VmTemplate.Context.IP)
	d.Set("permissions", permissionString(vm.Permissions))

	//Pull in NIC config from OpenNebula into schema
	if vm.VmTemplate.NICs != nil {
		d.Set("nic", flattenVmNICs(&vm.VmTemplate.NICs))
		d.Set("ip", &vm.VmTemplate.NICs[0].IP)
	}

	return nil
}

func flattenVmNICs(nics *[]VirtualMachineNIC) []interface{} {
	result := make([]interface{}, 0, len(*nics))
	for _, nic := range *nics {
		nicConfig := make(map[string]interface{})

		if nic.IP != "" {
			nicConfig["ip"] = nic.IP
		}
		if nic.MAC != "" {
			nicConfig["mac"] = nic.MAC
		}
		if nic.Model != "" {
			nicConfig["model"] = nic.Model
		}
		if nic.Network_ID != 0 {
			nicConfig["network_id"] = nic.Network_ID
		}
		if nic.NIC_ID != 0 {
			nicConfig["nic_id"] = nic.NIC_ID
		}
		if nic.Security_Groups != "" {
			nicConfig["security_groups"] = nic.Security_Groups
		}

		result = append(result, nicConfig)
	}
	return result
}

func resourceVmExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	err := resourceVmRead(d, meta)
	// a terminated VM is in state 6 (DONE)
	if err != nil || d.Id() == "" || d.Get("state").(int) == 6 {
		return false, err
	}

	return true, nil
}

func resourceVmUpdate(d *schema.ResourceData, meta interface{}) error {

	// Enable partial state mode
	d.Partial(true)

	client := meta.(*Client)

	if d.HasChange("permissions") && d.Get("permissions") != "" {
		resp, err := changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.vm.chmod")
		if err != nil {
			return err
		}
		d.SetPartial("permissions")
		log.Printf("[INFO] Successfully updated VM %s\n", resp)
	}

	// We succeeded, disable partial mode. This causes Terraform to save
	// save all fields again.
	d.Partial(false)

	return nil
}

func resourceVmDelete(d *schema.ResourceData, meta interface{}) error {
	err := resourceVmRead(d, meta)
	if err != nil || d.Id() == "" {
		return err
	}

	client := meta.(*Client)
	resp, err := client.Call("one.vm.action", "terminate-hard", intId(d.Id()))
	if err != nil {
		return err
	}

	_, err = waitForVmState(d, meta, "done")
	if err != nil {
		return fmt.Errorf(
			"Error waiting for virtual machine (%s) to be in state DONE: %s", d.Id(), err)
	}

	log.Printf("[INFO] Successfully terminated VM %s\n", resp)
	return nil
}

func waitForVmState(d *schema.ResourceData, meta interface{}, state string) (interface{}, error) {
	var vm *UserVm
	client := meta.(*Client)

	log.Printf("Waiting for VM (%s) to be in state Done", d.Id())

	stateConf := &resource.StateChangeConf{
		Pending: []string{"anythingelse"},
		Target:  []string{state},
		Refresh: func() (interface{}, string, error) {
			log.Println("Refreshing VM state...")
			if d.Id() != "" {
				resp, err := client.Call("one.vm.info", intId(d.Id()))
				if err == nil {
					if err = xml.Unmarshal([]byte(resp), &vm); err != nil {
						return nil, "", fmt.Errorf("Couldn't fetch VM state: %s", err)
					}
				} else {
					return nil, "", fmt.Errorf("Could not find VM by ID %s", d.Id())
				}
			}
			log.Printf("VM is currently in state %v and in LCM state %v", vm.State, vm.LcmState)
			if vm.State == 3 && vm.LcmState == 3 {
				return vm, "running", nil
			} else if vm.State == 6 {
				return vm, "done", nil
			} else if vm.State == 3 && vm.LcmState == 36 {
				errMsg := "No error was found"
				if vm.VmUserTemplate["ERROR"] != "" {
					errMsg = vm.VmUserTemplate["ERROR"]
				}
				return vm, "boot_failure", fmt.Errorf("VM ID %s entered fail state, error message: %s", d.Id(), errMsg)
			} else {
				return vm, "anythingelse", nil
			}
		},
		Timeout:    10 * time.Minute,
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	return stateConf.WaitForState()
}

func generateVmXML (d *schema.ResourceData) (string, error) {

	//Generate CONTEXT definition
	//context := d.Get("context").(*schema.Set).List()
	context := d.Get("context").(map[string]interface{})
	log.Printf("Number of CONTEXT vars: %d", len(context))
	log.Printf("CONTEXT Map: ", context)

	vmcontext := make(StringMap)
	for key, value := range context {
		//contextvar = v.(map[string]interface{})
		vmcontext[key] = fmt.Sprint(value)
	}


	//Generate NIC definition
	nics := d.Get("nic").(*schema.Set).List()
	log.Printf("Number of NICs: %d", len(nics))
	vmnics := make([]VirtualMachineNIC, len(nics))
	for i := 0; i < len(nics); i++ {
		nicconfig := nics[i].(map[string]interface{})
		nicip := nicconfig["ip"].(string)
		nicmodel := nicconfig["model"].(string)
		nicnetworkid := nicconfig["network_id"].(int)
		nicsecgroups := arrayToString(nicconfig["security_groups"].([]interface{}) , ",")

		vmnic := VirtualMachineNIC {
			IP:              nicip,
			Model:           nicmodel,
			Network_ID:      nicnetworkid,
			Security_Groups: nicsecgroups,
		}
		vmnics[i] = vmnic
	}

	//Generate DISK definition
	disks := d.Get("disk").(*schema.Set).List()
	log.Printf("Number of disks: %d", len(disks))
	vmdisks := make([]VirtualMachineDisk, len(disks))
	for i := 0; i < len(disks); i++ {
		diskconfig := disks[i].(map[string]interface{})
		diskimageid := diskconfig["image_id"].(int)
		disksize := diskconfig["size"].(int)
		disktarget := diskconfig["target"].(string)
		diskdriver := diskconfig["driver"].(string)

		vmdisk := VirtualMachineDisk {
			Image_ID:    diskimageid,
			Size:        disksize,
			Target:      disktarget,
			Driver:      diskdriver,
		}
		vmdisks[i] = vmdisk
	}

	//Generate GRAPHICS definition
	var vmgraphics VirtualMachineGraphics
	if g, ok := d.GetOk("graphics"); ok {
		graphics := g.(*schema.Set).List()
		graphicsconfig := graphics[0].(map[string]interface{})
		gfxlisten := graphicsconfig["listen"].(string)
		gfxtype := graphicsconfig["type"].(string)
		vmgraphics = VirtualMachineGraphics {
			Listen:      gfxlisten,
			Type:        gfxtype,
		}
	}

	//Generate OS definition
	var vmos VirtualMachineOS
	if o, ok := d.GetOk("os"); ok {
		os := o.(*schema.Set).List()
		osconfig := os[0].(map[string]interface{})
		osarch := osconfig["arch"].(string)
		osboot := osconfig["boot"].(string)
		vmos = VirtualMachineOS {
			Arch:        osarch,
			Boot:        osboot,
		}
	}
	//Generate RAW definition
	var vmraw VirtualMachineRAW
	if r, ok := d.GetOk("raw"); ok {
		raw := r.(*schema.Set).List()
		rawconfig := raw[0].(map[string]interface{})
		rawtype := rawconfig["type"].(string)
		rawdata := rawconfig["data"].(string)
		vmraw = VirtualMachineRAW {
			Type:        rawtype,
			Data:        rawdata,
		}
	}

	//Pull all the bits together into the main VM template
	vmname := d.Get("name").(string)
	vmvcpu := d.Get("vcpu").(int)
	vmcpu := d.Get("cpu").(float64)
	vmmemory := d.Get("memory").(int)

	vmtpl := &VmTemplate {
		Name:        vmname,
		VCPU:        vmvcpu,
		CPU:         vmcpu,
		Memory:      vmmemory,
		ContextVars: vmcontext,
		NICs:        vmnics,
		Disks:       vmdisks,
		Graphics:    vmgraphics,
		OS:          vmos,
		RAW:         vmraw,
	}

	w := &bytes.Buffer{}

	//Encode the VM template schema to XML
	enc := xml.NewEncoder(w)
	//enc.Indent("", "  ")
	if err := enc.Encode(vmtpl); err != nil {
		return "", err
	}

	log.Printf("VM XML: %s", w.String())
	return w.String(), nil

}

func arrayToString(a []interface{}, delim string) string {
    return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

func resourceVMNicHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})
	buf.WriteString(fmt.Sprintf("%s-", m["model"].(string)))
	buf.WriteString(fmt.Sprintf("%s-", m["network_id"].(int)))
	return hashcode.String(buf.String())
}

func resourceVMCustomizeDiff(diff *schema.ResourceDiff, v interface{}) error {
    // If the VM is in error state, force the VM to be recreated
    if diff.Get("lcmstate") == 36 {
        log.Printf("[INFO] VM is in error state, forcing recreate.")
        diff.SetNew("lcmstate", 3)
        if err := diff.ForceNew("lcmstate"); err != nil {
            return err
        }
    }

    return nil
}
