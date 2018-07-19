package opennebula

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strconv"
	"strings"
	"time"
	"bytes"
)

type Image struct {
	XMLName		xml.Name
	Name		string			`xml:"NAME"`
	Description	string			`xml:"DESCRIPTION,omitempty"`
	Id			int				`xml:"ID,omitempty"`
	Uid			int				`xml:"UID,omitempty"`
	Gid			int				`xml:"GID,omitempty"`
	Uname		string			`xml:"UNAME,omitempty"`
	Gname		string			`xml:"GNAME,omitempty"`
	Permissions	*Permissions	`xml:"PERMISSIONS,omitempty"`
	RegTime		string			`xml:"REG,omitempty"`
	Size		int				`xml:"SIZE,omitempty"`
	State		int				`xml:"STATE,omitempty"`
	Source		string			`xml:"SOURCE,omitempty"`
	Path		string			`xml:"PATH,omitempty"`
	Persistent	string			`xml:"PERSISTENT,omitempty"`
	DatastoreID	int				`xml:"DATASTORE_ID,omitempty"`
	Datastore	string			`xml:"DATASTORE,omitempty"`
	FsType		string			`xml:"FSTYPE,omitempty"`
	Type		string			`xml:"TYPE,omitempty"`
	DevPrefix	string			`xml:"DEV_PREFIX,omitempty"` //For image creation
	Target		string			`xml:"TARGET,omitempty"`  //For image creation
	Driver		string			`xml:"DRIVER,omitempty"` //For image creation
	Format		string			`xml:"FORMAT,omitempty"` //For image creation
	MD5			string			`xml:"MD5,omitempty"` //For image creation
	SHA1		string			`xml:"SHA1,omitempty"`	 //For image creation
	Template	*ImageTemplate	`xml:"TEMPLATE,omitempty"`
}

type Images struct {
	Image		[]*Image `xml:"IMAGE"`
}

type ImageTemplate struct {
	DevPrefix	string		`xml:"DEV_PREFIX,omitempty"`
	Driver		string	   `xml:"DRIVER,omitempty"`
	Format		string	   `xml:"FORMAT,omitempty"`
	MD5			string	   `xml:"MD5,omitempty"`
	SHA1		string	   `xml:"SHA1.omitempty"`

}

func resourceImage() *schema.Resource {
	return &schema.Resource{
		Create: resourceImageCreate,
		Read:   resourceImageRead,
		Exists: resourceImageExists,
		Update: resourceImageUpdate,
		Delete: resourceImageDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:			schema.TypeString,
				Required:		true,
				Description:	"Name of the Image",
			},
			"description": {
				Type:			schema.TypeString,
				Optional:		true,
				Description:	"Description of the Image, in OpenNebula's XML or String format",
			},
			"permissions": {
				Type:			schema.TypeString,
				Optional:		true,
				Computed:		true,
				Description:	"Permissions for the Image (in Unix format, owner-group-other, use-manage-admin)",
				ValidateFunc: 	func(v interface{}, k string) (ws []string, errors []error) {
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
				Description:	"ID of the user that will own the Image",
			},
			"gid": {
				Type:			schema.TypeInt,
				Computed:		true,
				Description:	"ID of the group that will own the Image",
			},
			"uname": {
				Type:			schema.TypeString,
				Computed:		true,
				Description:	"Name of the user that will own the Image",
			},
			"gname": {
				Type:			schema.TypeString,
				Computed:		true,
				Description:	"Name of the group that will own the Image",
			},
			"clone_from_image": {
				Type:			schema.TypeString,
				Optional:		true,
				ForceNew:		true,
				Description:	"ID or name of the Image to be cloned from",
				ConflictsWith:	[]string{"path"},
			},
			"datastore_id": {
				Type:			schema.TypeInt,
				Required:		true,
				ForceNew:		true,
				Description:	"ID of the datastore where Image will be stored",
			},
			"persistent": {
				Type:			schema.TypeBool,
				Optional:		true,
				Default:		false,
				ForceNew:		true,
				Description:	"Flag which indicates if the Image has to be persistent",
			},
			"path": {
				Type:			schema.TypeString,
				Optional:		true,
				Computed:		true,
				ForceNew:		true,
				Description:	"Path to the new image (local path on the OpenNebula server or URL)",
				ConflictsWith:	[]string{"clone_from_image"},
			},
			"type": {
				Type:			schema.TypeString,
				Optional:		true,
				Computed:		true,
				ForceNew:		true,
				Description:	"Type of the new Image: OS, CDROM, DATABLOCK, KERNEL, RAMDISK, CONTEXT",
				ValidateFunc: func (v interface{}, k string) (ws []string, errors []error) {
					validtypes := []string{"OS", "CDROM", "DATABLOCK", "KERNEL", "RAMDISK", "CONTEXT"}
					value := v.(string)

					if ! in_array(value, validtypes) {
						errors = append(errors, fmt.Errorf("Type %q must be one of: %s", k, strings.Join(validtypes,",")))
					}

					return
				},
			},
			"size": {
				Type:			schema.TypeInt,
				ForceNew:		true,
				Optional:		true,
				Computed:		true,
				Description:	"Size of the new image in MB",
			},
			"dev_prefix": {
				Type:			schema.TypeString,
				ForceNew:		true,
				Optional:		true,
				Computed:		true,
				Description:	"Device prefix, normally one of: hd, sd, vd",
			},
			"driver": {
				Type:			schema.TypeString,
				ForceNew:		true,
				Optional:		true,
				Computed:		true,
				Description:	"Driver to use, normally 'raw' or 'qcow2'",
			},
		},
	}
}

func resourceImageCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	// Check if Image ID for cloning is set
	if len(d.Get("clone_from_image").(string)) > 0 {
		return resourceImageClone(d, meta)
	} else { //Otherwise allocate a new image
		client := meta.(*Client)

		var resp string
		var err error

		imagexml, xmlerr := generateImageXML(d)
		if xmlerr != nil {
			return xmlerr
		}

		resp, err = client.Call(
			"one.image.allocate",
			imagexml,
			d.Get("datastore_id"),
		)

		if err != nil {
			return err
		}

		d.SetId(resp)
	}

	_, err := waitForImageState(d, meta, "ready")
	if err != nil {
		return fmt.Errorf("Error waiting for Image (%s) to be in state READY: %s", d.Id(), err)
	}

	// update permisions
	if _, ok := d.GetOk("permissions"); ok {
		if _, err = changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.image.chmod"); err != nil {
			return err
		}
	}

	return resourceImageRead(d, meta)
}

func resourceImageClone(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	var imageId int

	//Test if clone_from_image is an integer or not
	if val, err := strconv.Atoi(d.Get("clone_from_image").(string)); err == nil {
		imageId = val
	} else {
		imageId, err = getImageIdByName(d, meta)
		if err != nil {
			return fmt.Errorf("Unable to find Image by ID or name %s", d.Get("clone_from_image"))
		}
	}

	// Clone Image from given ID
	resp, err := client.Call(
		"one.image.clone",
		imageId,
		d.Get("name"),
		d.Get("datastore_id"),
	)
	if err != nil {
		return err
	}

	d.SetId(resp)

	_, err = waitForImageState(d, meta, "ready")
	if err != nil {
		return fmt.Errorf("Error waiting for Image (%s) to be in state READY: %s", d.Id(), err)
	}

	// update permisions
	if _, ok := d.GetOk("permissions"); ok {
		if _, err = changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.image.chmod"); err != nil {
			return err
		}
	}

	// set persistency if needed
	resp, err = client.Call(
		"one.image.persistent",
		intId(d.Id()),
		d.Get("persistent"),
	)
	if err != nil {
		return err
	}

	return resourceImageRead(d, meta)
}

func waitForImageState(d *schema.ResourceData, meta interface{}, state string) (interface{}, error) {
	var img *Image
	client := meta.(*Client)

	stateConf := &resource.StateChangeConf{
		Pending: []string{"anythingelse"},
		Target:  []string{state},
		Refresh: func() (interface{}, string, error) {
			log.Println("Refreshing Image state...")
			if d.Id() != "" {
				resp, err := client.Call("one.image.info", intId(d.Id()))
				if err == nil {
					if err = xml.Unmarshal([]byte(resp), &img); err != nil {
						return nil, "", fmt.Errorf("Couldn't fetch Image state: %s", err)
					}
				} else {
					log.Printf("Image %v was not found", d.Id())
					//We can't return nil or Terraform will keep waiting
					//forever, so return an empty struct
					img := &Image{}
					return img, "notfound", nil
				}
			}
			log.Printf("Image %v is currently in state %v", img.Id, img.State)
			if img.State == 1 {
				return img, "ready", nil
			} else if img.State == 5 {
				return img, "error", fmt.Errorf("Image ID %v entered error state.", d.Id())
			} else {
				return img, "anythingelse", nil
			}
		},
		Timeout:	10 * time.Minute,
		Delay:		10 * time.Second,
		MinTimeout:	3 * time.Second,
	}

	return stateConf.WaitForState()
}

func resourceImageRead(d *schema.ResourceData, meta interface{}) error {
	var img *Image
	var imgs *Images

	image_type_id_name := map[int]string {
		0: "OS",
		1: "CDROM",
		2: "DATABLOCK",
		3: "KERNEL",
		4: "RAMDISK",
		5: "CONTEXT",
	}

	client := meta.(*Client)
	found := false

	// Try to find the Image by ID, if specified
	if d.Id() != "" {
		resp, err := client.Call("one.image.info", intId(d.Id()), false)
		if err == nil {
			found = true
			if err = xml.Unmarshal([]byte(resp), &img); err != nil {
				return err
			}
		} else {
			log.Printf("Could not find Image by ID %s", d.Id())
		}
	}

	// Otherwise, try to find the Image by (user, name) as the de facto compound primary key
	if d.Id() == "" || !found {
		resp, err := client.Call("one.imagepool.info", -2, -1, -1)
		if err != nil {
			return err
		}

		if err = xml.Unmarshal([]byte(resp), &imgs); err != nil {
			return err
		}

		for _, t := range imgs.Image {
			if t.Name == d.Get("name").(string) {
				img = t
				found = true
				break
			}
		}

		if !found || img == nil {
			d.SetId("")
			log.Printf("Could not find Image with name %s for user %s", d.Get("name").(string), client.Username)
			return nil
		}
	}

	d.SetId(strconv.Itoa(img.Id))
	d.Set("name", img.Name)
	d.Set("uid", img.Uid)
	d.Set("gid", img.Gid)
	d.Set("uname", img.Uname)
	d.Set("gname", img.Gname)
	d.Set("permissions", permissionString(img.Permissions))
	d.Set("persistent", img.Persistent)
	d.Set("path", img.Path)

	if imgtypeint, err := strconv.Atoi(img.Type); err == nil {
		if val, ok := image_type_id_name[imgtypeint]; ok {
			d.Set("type", val)
		}
	}

	d.Set("size", img.Size)
	d.Set("dev_prefix", img.Template.DevPrefix)
	d.Set("driver", img.Template.Driver)

	return nil
}

func getImageIdByName(d *schema.ResourceData, meta interface{}) (int, error) {
	var img *Image
	var imgs *Images

	client := meta.(*Client)
	found := false

	resp, err := client.Call("one.imagepool.info", -3, -1, -1)
	if err != nil {
		return 0, err
	}

	if err = xml.Unmarshal([]byte(resp), &imgs); err != nil {
		return 0, err
	}

	for _, t := range imgs.Image {
		if t.Name == d.Get("clone_from_image").(string) {
			img = t
			found = true
			break
		}
	}

	if !found || img == nil {
		log.Printf("Could not find Image with name %s for user %s", d.Get("clone_from_image").(string), client.Username)
		err = errors.New("ImageNotFound")
		return 0, err
	}

	return img.Id, nil
}

func resourceImageExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	err := resourceImageRead(d, meta)
	if err != nil || d.Id() == "" {
		return false, err
	}

	return true, nil
}

func resourceImageUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	if d.HasChange("description") {
		_, err := client.Call(
			"one.image.update",
			intId(d.Id()),
			d.Get("description").(string),
			0, // replace the whole image instead of merging it with the existing one
		)
		if err != nil {
			return err
		}
	}

	if d.HasChange("name") {
		resp, err := client.Call(
			"one.image.rename",
			intId(d.Id()),
			d.Get("name").(string),
		)
		if err != nil {
			return err
		}
		log.Printf("[INFO] Successfully updated name for Image %s\n", resp)
	}

	if d.HasChange("permissions") {
		resp, err := changePermissions(intId(d.Id()), permission(d.Get("permissions").(string)), client, "one.image.chmod")
		if err != nil {
			return err
		}
		log.Printf("[INFO] Successfully updated Image %s\n", resp)
	}

	return nil
}

func resourceImageDelete(d *schema.ResourceData, meta interface{}) error {
	err := resourceImageRead(d, meta)
	if err != nil || d.Id() == "" {
		return err
	}

	client := meta.(*Client)

	resp, err := client.Call("one.image.delete", intId(d.Id()), false)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Successfully deleted Image %s\n", resp)

	_, err = waitForImageState(d, meta, "notfound")
	if err != nil {
		return fmt.Errorf("Error waiting for Image (%s) to be in state NOTFOUND: %s", d.Id(), err)
	}

	return nil
}



func generateImageXML(d *schema.ResourceData) (string, error) {

	var imagedescription string
	var imagetype string
	var imagesize int
	var imagedevprefix string
	var imagepersistent string
	var imagetarget string
	var imagedriver string
	var imagepath string
	//var imagedisktype string
	var imagemd5 string
	var imagesha1 string

	imagename := d.Get("name").(string)

	if val, ok := d.GetOk("description"); ok {
		imagedescription = val.(string)
	}

	if val, ok := d.GetOk("type"); ok {
		imagetype = val.(string)
	}

	if d.Get("persistent") != nil {
		imagepersistent = "NO"
		if d.Get("persistent") == true {
			imagepersistent = "YES"
		}
	}

	if val, ok := d.GetOk("size"); ok {
		imagesize = val.(int)
	}

	if val, ok := d.GetOk("dev_prefix"); ok {
		imagedevprefix = val.(string)
	}

	if val, ok := d.GetOk("target"); ok {
		imagetarget = val.(string)
	}

	if val, ok := d.GetOk("driver"); ok {
		imagedriver = val.(string)
	}

	if val, ok := d.GetOk("path"); ok {
		imagepath = val.(string)
	}

	if val, ok := d.GetOk("md5"); ok {
		imagemd5 = val.(string)
	}

	if val, ok := d.GetOk("sha1"); ok {
		imagesha1 = val.(string)
	}

	imagetpl := &Image {
		Name:				imagename,
		Description: 		imagedescription,
		Size:				imagesize,
		Type:				imagetype,
		Persistent:			imagepersistent,
		DevPrefix:			imagedevprefix,
		Target:				imagetarget,
		Driver:				imagedriver,
		Path:				imagepath,
		MD5:				imagemd5,
		SHA1:				imagesha1,
	}

	imagetpl.XMLName.Local = "IMAGE"

	w := &bytes.Buffer{}

	//Encode the Security Group template schema to XML
	enc := xml.NewEncoder(w)
	if err := enc.Encode(imagetpl); err != nil {
		return "", err
	}

	log.Printf("[INFO] Image Definition XML: %s", w.String())
	return w.String(), nil
}
